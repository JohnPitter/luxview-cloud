package detector

import (
	"path/filepath"
	"strings"

	"github.com/luxview/engine/internal/agent"
)

func detectServices(repoDir string) []agent.ServiceRecommendation {
	var recs []agent.ServiceRecommendation
	seen := make(map[string]bool)

	pkg := readFile(repoDir, "package.json")
	goMod := readFile(repoDir, "go.mod")
	reqs := readFile(repoDir, "requirements.txt")
	prisma := readFile(repoDir, filepath.Join("prisma", "schema.prisma"))
	compose := readFile(repoDir, "docker-compose.yml") + readFile(repoDir, "docker-compose.yaml")

	if (strings.Contains(prisma, "postgresql") ||
		strings.Contains(pkg, `"pg"`) ||
		strings.Contains(pkg, `"postgres"`) ||
		strings.Contains(goMod, "github.com/lib/pq") ||
		strings.Contains(goMod, "github.com/jackc/pgx") ||
		strings.Contains(reqs, "psycopg2") ||
		strings.Contains(reqs, "asyncpg") ||
		strings.Contains(compose, "postgres")) && !seen["postgres"] {
		seen["postgres"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "PostgreSQL",
			CurrentEvidence:    detectPostgresEvidence(pkg, prisma, goMod, reqs, compose),
			RecommendedService: "postgres",
			Reason:             "Managed PostgreSQL with automatic backups",
			ManualSteps: []string{
				"Set DATABASE_URL in environment variables",
				"Run database migrations",
				"Verify application connects correctly",
			},
		})
	}

	if (strings.Contains(pkg, `"redis"`) ||
		strings.Contains(pkg, `"ioredis"`) ||
		strings.Contains(goMod, "github.com/redis/go-redis") ||
		strings.Contains(goMod, "github.com/go-redis/redis") ||
		strings.Contains(reqs, "redis") ||
		strings.Contains(compose, "redis")) && !seen["redis"] {
		seen["redis"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "Redis",
			CurrentEvidence:    "Detected in project dependencies",
			RecommendedService: "redis",
			Reason:             "Managed Redis for caching and sessions",
			ManualSteps: []string{
				"Set REDIS_URL in environment variables",
				"Verify cache/session functionality",
			},
		})
	}

	if (strings.Contains(pkg, `"mongoose"`) ||
		strings.Contains(pkg, `"mongodb"`) ||
		strings.Contains(goMod, "go.mongodb.org/mongo-driver") ||
		strings.Contains(reqs, "pymongo") ||
		strings.Contains(compose, "mongo")) && !seen["mongodb"] {
		seen["mongodb"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "MongoDB",
			CurrentEvidence:    "Detected in project dependencies",
			RecommendedService: "mongodb",
			Reason:             "Managed MongoDB instance",
			ManualSteps: []string{
				"Set MONGODB_URL in environment variables",
				"Verify database connectivity",
			},
		})
	}

	if (strings.Contains(pkg, `"amqplib"`) ||
		strings.Contains(goMod, "github.com/rabbitmq/amqp091-go") ||
		strings.Contains(reqs, "pika") ||
		strings.Contains(compose, "rabbitmq")) && !seen["rabbitmq"] {
		seen["rabbitmq"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "RabbitMQ",
			CurrentEvidence:    "Detected in project dependencies",
			RecommendedService: "rabbitmq",
			Reason:             "Managed message queue",
			ManualSteps: []string{
				"Set RABBITMQ_URL in environment variables",
				"Verify queue connectivity",
			},
		})
	}

	return recs
}

func detectPostgresEvidence(pkg, prisma, goMod, reqs, compose string) string {
	if strings.Contains(prisma, "postgresql") {
		return "prisma/schema.prisma: provider = postgresql"
	}
	if strings.Contains(pkg, `"pg"`) {
		return "package.json: pg dependency"
	}
	if strings.Contains(pkg, `"postgres"`) {
		return "package.json: postgres dependency"
	}
	if strings.Contains(goMod, "pgx") || strings.Contains(goMod, "lib/pq") {
		return "go.mod: PostgreSQL driver"
	}
	if strings.Contains(reqs, "psycopg2") || strings.Contains(reqs, "asyncpg") {
		return "requirements.txt: PostgreSQL driver"
	}
	return "docker-compose: postgres service"
}
