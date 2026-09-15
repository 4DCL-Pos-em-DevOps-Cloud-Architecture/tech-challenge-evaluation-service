# evaluation-service (Go)

Este é o serviço de avaliação, o "caminho quente" (hot path) do projeto ToggleMaster. É o único endpoint que os clientes finais (ex: seu app mobile, seu site) devem chamar.

Ele é otimizado para alta velocidade e baixa latência usando **cache em Redis**.

Ele funciona da seguinte forma:
1.  Recebe uma requisição (`/evaluate?user_id=...&flag_name=...`).
2.  Busca as regras da flag no **Redis**.
3.  **Se não estiver no cache (Cache MISS):**
    * Busca a definição da flag no `flag-service`.
    * Busca a regra no `targeting-service`.
    * Salva o resultado no Redis com um TTL (Time-To-Live) curto.
4.  Executa a lógica de avaliação (ex: "o usuário está nos 50%?").
5.  Retorna `true` ou `false` para o cliente.
6.  Envia *assincronamente* um evento da decisão para uma fila **AWS SQS**.

## 📦 Pré-requisitos (Local)

* [Go](https://go.dev/doc/install) (versão 1.21 ou superior)
* [Redis](https://redis.io/docs/getting-started/installation/) (rodando localmente ou em Docker)
* Os serviços `auth-service`, `flag-service` e `targeting-service` devem estar rodando.
* **Credenciais da AWS:** Para o SQS funcionar, seu terminal deve estar autenticado na AWS (ex: via `aws configure` ou variáveis de ambiente).

## 🚀 Rodando Localmente

1.  **Clone o repositório** e entre na pasta `evaluation-service`.

2.  **Crie uma Chave de API de Serviço:**
    Este serviço precisa se autenticar no `flag-service` e no `targeting-service`. Você deve criar uma chave de API para ele usando o `auth-service` (com a `MASTER_KEY`).
    ```bash
    curl -X POST http://localhost:8001/admin/keys \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer admin-secreto-123" \
    -d '{"name": "evaluation-service-key"}'
    ```
    Guarde a chave `key` retornada (ex: `tm_key_...`). Vamos chamá-la de `SUA_CHAVE_DE_SERVICO`.

3.  **Configure as Variáveis de Ambiente:**
    Crie um arquivo chamado `.env` na raiz desta pasta com o seguinte conteúdo:
    ```.env
    # Porta que este serviço irá rodar
    PORT="8004"
    
    # URL do seu Redis local
    REDIS_URL="redis://localhost:6379"
    
    # URLs dos outros serviços
    FLAG_SERVICE_URL="http://localhost:8002"
    TARGETING_SERVICE_URL="http://localhost:8003"
    
    # Chave de API que você criou no passo 2
    SERVICE_API_KEY="SUA_CHAVE_DE_SERVICO"
    
    # --- Configuração da AWS (Obrigatório para o desafio) ---
    # Cole a URL da fila SQS que você criou no console da AWS
    AWS_SQS_URL="[https://sqs.us-east-1.amazonaws.com/123456789012/sua-fila](https://sqs.us-east-1.amazonaws.com/123456789012/sua-fila)"
    
    # Região da sua fila SQS
    AWS_REGION="us-east-1" 
    ```

4.  **Instale as Dependências:**
    ```bash
    go mod tidy
    ```

5.  **Inicie o Serviço:**
    ```bash
    go run .
    ```
    O servidor estará rodando em `http://localhost:8004`.

## 🧪 Testando os Endpoints

Para os testes, vamos assumir que você já criou:
1.  Uma flag chamada `enable-new-dashboard` no `flag-service`.
2.  Uma regra para `enable-new-dashboard` no `targeting-service` do tipo `PERCENTAGE` com valor `50`.

**1. Verifique a Saúde (Health Check):**
```bash
curl http://localhost:8004/health
```
Saída esperada: `{"status":"ok"}``

**2. Teste a Avaliação:** Tente alguns IDs de usuário diferentes. O hash determinístico fará com que alguns caiam dentro dos 50% e outros fora.

```bash
# Teste User 1
curl "http://localhost:8004/evaluate?user_id=user-123&flag_name=enable-new-dashboard"
```
Saída (exemplo): `{"flag_name":"enable-new-dashboard","user_id":"user-123","result":true}`

```bash
# Teste User 2
curl "http://localhost:8004/evaluate?user_id=user-abc&flag_name=enable-new-dashboard"
```
Saída (exemplo): `{"flag_name":"enable-new-dashboard","user_id":"user-abc","result":false}`

**3. Verifique o Cache:** Execute o mesmo comando duas vezes seguidas. Na segunda vez, você verá um log "Cache HIT" no terminal do `evaluation-service`.

**4. Verifique a Fila SQS:** Após fazer as chamadas acima, vá até o console da AWS, abra sua fila SQS e verifique se as mensagens (`EvaluationEvent`) estão chegando.

