package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Struct para a resposta da BrasilAPI
type BrasilAPIResponse struct {
	CEP        string `json:"cep"`
	Estado     string `json:"state"`
	Cidade     string `json:"city"`
	Bairro     string `json:"neighborhood"`
	Logradouro string `json:"street"`
	Servico    string `json:"service"`
}

// Struct para a resposta da ViaCEP
type ViaCEPResponse struct {
	CEP         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	UF          string `json:"uf"`
	Erro        bool   `json:"erro"`
}

func main() {
	cep := "52020-060"

	chBrasilAPI := make(chan string)
	chViaCEP := make(chan string)

	go buscaBrasilAPI(cep, chBrasilAPI)
	go buscaViaCEP(cep, chViaCEP)

	select {
	case resultado := <-chBrasilAPI:
		fmt.Println("API: BrasilAPI (Venceu!)")
		fmt.Println(resultado)

	case resultado := <-chViaCEP:
		fmt.Println("API: ViaCEP (Venceu!)")
		fmt.Println(resultado)

	case <-time.After(1 * time.Second):
		fmt.Println("Erro: Timeout de 1 segundo atingido.")
	}
}

func buscaBrasilAPI(cep string, ch chan<- string) {
	url := "https://brasilapi.com.br/api/cep/v1/" + cep

	res, err := http.Get(url)

	if err != nil {
		fmt.Printf("BrasilAPI - Erro HTTP: %v\n", err)
		return
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		fmt.Printf("BrasilAPI - Erro ao ler corpo: %v\n", err)
		return
	}

	var data BrasilAPIResponse

	err = json.Unmarshal(body, &data)

	if err != nil {
		fmt.Printf("BrasilAPI - Erro ao decodificar JSON: %v\n", err)
		return
	}

	if data.CEP == "" {
		fmt.Println("BrasilAPI - CEP não encontrado")
		return
	}

	resultado := fmt.Sprintf("CEP: %s\nLogradouro: %s\nBairro: %s\nCidade: %s\nEstado: %s",
		data.CEP, data.Logradouro, data.Bairro, data.Cidade, data.Estado)

	ch <- resultado
}

func buscaViaCEP(cep string, ch chan<- string) {
	url := "http://viacep.com.br/ws/" + cep + "/json/"

	res, err := http.Get(url)

	if err != nil {
		fmt.Printf("ViaCEP - Erro HTTP: %v\n", err)
		return
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		fmt.Printf("ViaCEP - Erro ao ler corpo: %v\n", err)
		return
	}

	var data ViaCEPResponse

	err = json.Unmarshal(body, &data)

	if err != nil {
		fmt.Printf("ViaCEP - Erro ao decodificar JSON: %v\n", err)
		return
	}

	if data.Erro {
		fmt.Println("ViaCEP - CEP não encontrado (erro: true)")
		return
	}

	resultado := fmt.Sprintf("CEP: %s\nLogradouro: %s\nBairro: %s\nCidade: %s\nEstado: %s",
		data.CEP, data.Logradouro, data.Bairro, data.Localidade, data.UF)

	ch <- resultado
}
