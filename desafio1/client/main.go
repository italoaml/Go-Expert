package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type CotacaoResponse struct {
	Bid string `json:"bid"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)

	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/cotacao", nil)

	if err != nil {
		fmt.Printf("Erro ao criar requisição: %v", err)
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Println("Erro: Timeout no cliente (300ms) estourou")
		} else {
			fmt.Printf("Erro ao fazer requisição ao servidor: %v\n", err)
		}

		return
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		fmt.Printf("Erro ao ler resposta do servidor: %v", err)
	}

	var cotacao CotacaoResponse

	err = json.Unmarshal(body, &cotacao)

	if err != nil {
		fmt.Printf("Erro ao decodificar JSON da resposta: %v", err)
	}

	conteudoArquivo := fmt.Sprintf("Dólar: %s", cotacao.Bid)

	err = os.WriteFile("cotacao.txt", []byte(conteudoArquivo), 0644)

	if err != nil {
		fmt.Printf("Erro ao salvar arquivo cotacao.txt: %v", err)
	}

	fmt.Println("Arquivo cotacao.txt salvo com sucesso!")
	fmt.Printf("Cotação atual: Dólar: %s\n", cotacao.Bid)
}
