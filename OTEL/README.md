# Sistema de Clima por CEP com OpenTelemetry e Zipkin

Este projeto consiste em um sistema distribuído com dois microsserviços em Go que trabalham juntos para fornecer a temperatura atual de uma cidade a partir de um CEP.

Toda a comunicação entre os serviços é instrumentada com **OpenTelemetry** para gerar traces distribuídos, que são enviados para um **OTEL Collector** e podem ser visualizados no **Zipkin**.

## Arquitetura

O sistema é composto por 4 contêineres Docker:

1.  **`service-a`** (Porta `8080`): Serviço de entrada. Recebe o CEP, valida, e repassa a requisição para o `service-b`.
2.  **`service-b`** (Porta `8081`): Serviço de orquestração. Recebe o CEP, busca a cidade na API ViaCEP, busca o clima na WeatherAPI e retorna o resultado.
3.  **`otel-collector`**: Coleta os traces gerados pelos serviços `a` e `b`.
4.  **`zipkin`** (Porta `9411`): Backend para armazenamento e visualização dos traces.

**Fluxo da Requisição:**
`Cliente` → `service-a` → `service-b` → `(ViaCEP & WeatherAPI)`

**Fluxo dos Traces:**
`(service-a & service-b)` → `otel-collector` → `zipkin`

## Pré-requisitos

-   [Docker](https://www.docker.com/products/docker-desktop/)
-   [Docker Compose](https://docs.docker.com/compose/install/)

## ⚙️ Configuração

Antes de iniciar, você precisa de uma chave de API do site [WeatherAPI.com](https://www.weatherapi.com/).

1.  Crie um arquivo chamado `.env` na raiz do projeto.
2.  Dentro deste arquivo, adicione sua chave de API da seguinte forma:

    ```
    WEATHER_API_KEY=SUA_CHAVE_API_AQUI
    ```

    Substitua `SUA_CHAVE_API_AQUI` pela sua chave real.

## 🚀 Como Executar

Com o Docker em execução e o arquivo `.env` configurado, basta um comando para subir toda a aplicação:

```bash
docker compose up --build -d
```

O `-d` no final executa os contêineres em modo "detached" (em segundo plano).

## 🧪 Como Testar

Use um cliente HTTP como o `curl` para interagir com o `service-a` na porta `8080`.

#### Requisição de Sucesso
```bash
curl -X POST -H "Content-Type: application/json" -d '{"cep": "01001000"}' http://localhost:8080/
```

#### CEP com Formato Inválido
```bash
curl -X POST -H "Content-Type: application/json" -d '{"cep": "123"}' http://localhost:8080/
```
*Resposta esperada: `invalid zipcode` (Status 422)*

#### CEP Não Encontrado
```bash
curl -X POST -H "Content-Type: application/json" -d '{"cep": "99999999"}' http://localhost:8080/
```
*Resposta esperada: `can not find zipcode` (Status 404)*

## 📈 Observabilidade (Visualizando os Traces)

1.  Após fazer algumas requisições, abra o Zipkin no seu navegador: **[http://localhost:9411](http://localhost:9411)**.
2.  A tela principal de busca será exibida. Clique no botão azul **"Run Query"** no canto superior direito para buscar os traces mais recentes.
3.  Se necessário, ajuste o filtro de tempo para um período maior (ex: `1h`).
4.  Clique em um trace da lista para ver o detalhamento completo da comunicação entre `service-a` e `service-b`, incluindo os spans customizados.

## 🛑 Parando a Aplicação

Para parar e remover todos os contêineres, execute:

```bash
docker compose down
```