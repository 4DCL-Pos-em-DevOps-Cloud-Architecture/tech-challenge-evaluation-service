package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

// Contexto global para o Redis
var ctx = context.Background()

// App struct para injeção de dependência
type App struct {
	RedisClient         *redis.Client
	SqsSvc              *sqs.SQS
	SqsQueueURL         string
	HttpClient          *http.Client
	FlagServiceURL      string
	TargetingServiceURL string
}

func main() {
	_ = godotenv.Load() // Carrega .env para dev local

	// --- Configuração ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8004"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatalf("PORT inválida: %s", safeLogValue(port))
	}
	port = strconv.Itoa(portNumber)

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL deve ser definida (ex: redis://localhost:6379)")
	}

	flagSvcURL := os.Getenv("FLAG_SERVICE_URL")
	if flagSvcURL == "" {
		log.Fatal("FLAG_SERVICE_URL deve ser definida")
	}

	targetingSvcURL := os.Getenv("TARGETING_SERVICE_URL")
	if targetingSvcURL == "" {
		log.Fatal("TARGETING_SERVICE_URL deve ser definida")
	}

	// SQS é opcional no dev local, mas obrigatório em prod
	sqsQueueURL := os.Getenv("AWS_SQS_URL")
	awsRegion := os.Getenv("AWS_REGION")
	sqsEndpointURL := os.Getenv("AWS_SQS_ENDPOINT_URL")
	if sqsQueueURL == "" {
		log.Println("Atenção: AWS_SQS_URL não definida. Eventos não serão enviados.")
	}
	if awsRegion == "" && sqsQueueURL != "" {
		log.Fatal("AWS_REGION deve ser definida para usar SQS")
	}

	// --- Inicializa Clientes ---

	// Cliente Redis
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Não foi possível parsear a URL do Redis: %s", safeLogValue(err.Error()))
	}
	rdb := redis.NewClient(opt)
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("Não foi possível conectar ao Redis: %s", safeLogValue(err.Error()))
	}
	log.Println("Conectado ao Redis com sucesso!")

	// Cliente SQS (AWS SDK)
	var sqsSvc *sqs.SQS
	if sqsQueueURL != "" {
		config := &aws.Config{Region: aws.String(awsRegion)}
		if sqsEndpointURL != "" {
			config.Endpoint = aws.String(sqsEndpointURL)
			config.DisableSSL = aws.Bool(true)
		}

		sess, err := session.NewSession(config)
		if err != nil {
			log.Fatalf("Não foi possível criar sessão AWS: %s", safeLogValue(err.Error()))
		}
		sqsSvc = sqs.New(sess)
		log.Println("Cliente SQS inicializado com sucesso.")

		if sqsEndpointURL != "" {
			queueName := path.Base(sqsQueueURL)
			if queueName == "" || queueName == "." || queueName == "/" {
				log.Fatalf("Não foi possível derivar o nome da fila a partir de AWS_SQS_URL: %s", safeLogValue(sqsQueueURL))
			}

			var ensureErr error
			for attempt := 1; attempt <= 30; attempt++ {
				_, ensureErr = sqsSvc.CreateQueue(&sqs.CreateQueueInput{
					QueueName: aws.String(queueName),
				})
				if ensureErr == nil {
					break
				}
				log.Printf("Aguardando SQS local ficar pronto (%d/30): %s", attempt, safeLogValue(ensureErr.Error()))
				time.Sleep(1 * time.Second)
			}
			if ensureErr != nil {
				log.Fatalf("Não foi possível garantir a fila SQS local '%s': %s", safeLogValue(queueName), safeLogValue(ensureErr.Error()))
			}
			log.Printf("Fila SQS local garantida: %s", safeLogValue(queueName))
		}
	}

	// Cliente HTTP (com timeout)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Cria a instância da App
	app := &App{
		RedisClient:         rdb,
		SqsSvc:              sqsSvc,
		SqsQueueURL:         sqsQueueURL,
		HttpClient:          httpClient,
		FlagServiceURL:      flagSvcURL,
		TargetingServiceURL: targetingSvcURL,
	}

	// --- Rotas ---
	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/evaluate", app.evaluationHandler)

	log.Printf("Serviço de Avaliação (Go) rodando na porta %s", safeLogValue(port))
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(safeLogValue(err.Error()))
	}
}
