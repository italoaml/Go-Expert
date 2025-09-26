package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var tracer trace.Tracer

func initTracerProvider(ctx context.Context, serviceName, collectorURL string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	conn, err := grpc.NewClient(collectorURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))

	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider.Shutdown, nil
}

type ViaCEPResponse struct {
	Localidade string `json:"localidade"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type WeatherResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

func main() {
	ctx := context.Background()
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	collectorURL := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	shutdown, err := initTracerProvider(ctx, serviceName, collectorURL)

	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}

	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown tracer provider: %v", err)
		}
	}()

	tracer = otel.Tracer(serviceName)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Post("/", HandleWeatherRequest)

	log.Println("Service B started on port 8081")
	http.ListenAndServe(":8081", otelhttp.NewHandler(r, "service-b-server"))
}

func HandleWeatherRequest(w http.ResponseWriter, r *http.Request) {
	var requestBody map[string]string

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cep := requestBody["cep"]

	if !isValidCEP(cep) {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	// --- Busca da Cidade (com Span) ---
	ctx, citySpan := tracer.Start(r.Context(), "find-city-by-cep")
	cityName, err := findCityByCEP(ctx, cep)

	if err != nil {
		if err.Error() == "can not find zipcode" {
			http.Error(w, "can not find zipcode", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		citySpan.End()
		return
	}

	citySpan.End()

	// --- Busca da Temperatura (com Span) ---
	ctx, tempSpan := tracer.Start(ctx, "get-temperature-by-city")
	tempC, err := getTemperatureByCity(ctx, cityName)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		tempSpan.End()
		return
	}

	tempSpan.End()

	tempF := tempC*1.8 + 32
	tempK := tempC + 273

	response := WeatherResponse{
		City:  cityName,
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func isValidCEP(cep string) bool {
	match, _ := regexp.MatchString(`^\d{8}$`, cep)
	return match
}

func findCityByCEP(ctx context.Context, cep string) (string, error) {
	_, span := tracer.Start(ctx, "call-viacep-api")
	defer span.End()

	span.SetAttributes(attribute.String("cep", cep))

	req, err := http.NewRequestWithContext(ctx, "GET", "http://viacep.com.br/ws/"+cep+"/json/", nil)

	if err != nil {
		return "", err
	}

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	var viaCEPResponse ViaCEPResponse

	if err := json.NewDecoder(resp.Body).Decode(&viaCEPResponse); err != nil {
		return "", err
	}

	if viaCEPResponse.Localidade == "" {
		return "", fmt.Errorf("can not find zipcode")
	}

	return viaCEPResponse.Localidade, nil
}

func getTemperatureByCity(ctx context.Context, cityName string) (float64, error) {
	_, span := tracer.Start(ctx, "call-weather-api")
	defer span.End()

	span.SetAttributes(attribute.String("city", cityName))
	apiKey := os.Getenv("WEATHER_API_KEY")

	encodedCityName := url.QueryEscape(cityName)

	url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, encodedCityName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)

	if err != nil {
		return 0, err
	}

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	resp, err := client.Do(req)

	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var weatherAPIResponse WeatherAPIResponse

	if err := json.NewDecoder(resp.Body).Decode(&weatherAPIResponse); err != nil {
		return 0, err
	}

	return weatherAPIResponse.Current.TempC, nil
}
