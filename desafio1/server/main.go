package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const URL = "https://economia.awesomeapi.com.br/json/last/USD-BRL"

type APIResponse struct {
	USDBRL Cotacao `json:"USDBRL"`
}

type Cotacao struct {
	Bid string `json:"bid"`
}

func main() {
	if err := initDB(); err != nil {
		fmt.Printf("Erro ao inicializar banco de dados: %v", err)
	}

	http.HandleFunc("/cotacao", ConsultarCotacao)
	fmt.Println("Servidor rodando na porta 8080...")
	http.ListenAndServe(":8080", nil)
}

func ConsultarCotacao(w http.ResponseWriter, r *http.Request) {
	ctxAPI, cancelAPI := context.WithTimeout(r.Context(), 200*time.Millisecond)
	defer cancelAPI() // Sempre chame cancel() para liberar recursos

	cotacao, err := BuscarCotacaoAPI(ctxAPI)

	if err != nil {
		// Verificamos se o erro foi de timeout
		if err == context.DeadlineExceeded {
			fmt.Println("Erro: Timeout ao buscar cotação da API (200ms)")
			http.Error(w, "Timeout ao buscar cotação da API", http.StatusRequestTimeout)
			return
		}
		// Outros erros
		fmt.Printf("Erro ao buscar cotação: %v\n", err)
		http.Error(w, "Erro ao buscar cotação", http.StatusInternalServerError)
		return
	}

	ctxDB, cancelDB := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelDB()

	err = InserirDB(ctxDB, cotacao.Bid)

	if err != nil {
		// Verificamos se o erro foi de timeout
		if err == context.DeadlineExceeded {
			fmt.Println("Erro: Timeout ao salvar no banco de dados (10ms)")
			// O desafio não diz para parar a requisição se o DB falhar
			// Então apenas logamos, mas continuamos para responder ao cliente
		} else {
			fmt.Printf("Erro ao salvar no banco de dados: %v\n", err)
		}
	}

	responseMap := map[string]string{"bid": cotacao.Bid}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(responseMap)
}

func BuscarCotacaoAPI(ctx context.Context) (*Cotacao, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", URL, nil)

	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse

	err = json.Unmarshal(body, &apiResponse)

	if err != nil {
		return nil, err
	}

	return &apiResponse.USDBRL, nil
}

func initDB() error {
	db, err := sql.Open("sqlite3", "cotacao.db")
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS cotacao (id INTEGER PRIMARY KEY AUTOINCREMENT, bid TEXT, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP)")
	if err != nil {
		return err
	}
	return nil
}

func InserirDB(ctx context.Context, bid string) error {
	db, err := sql.Open("sqlite3", "cotacao.db")

	if err != nil {
		return err
	}

	defer db.Close()

	_, err = db.ExecContext(ctx, "INSERT INTO cotacao (bid) VALUES (?)", bid)

	if err != nil {
		return err
	}

	return nil
}
