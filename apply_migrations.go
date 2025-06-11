package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"

	_ "github.com/lib/pq"
)

func main() {
	// Database connection string
	connStr := "postgresql://trading_user:IcXgaLtKECYFHsHBK7EwO27V3C6lRGmK@ep-wispy-mountain-a53ywx4b.us-east-2.aws.neon.tech/trading_db"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Check if migrations table exists
	createMigrationsTable := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT NOW()
		);
	`
	_, err = db.Exec(createMigrationsTable)
	if err != nil {
		log.Fatal("Failed to create migrations table:", err)
	}

	// Get list of migration files
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		log.Fatal("Failed to read migrations directory:", err)
	}
	sort.Strings(files)

	for _, file := range files {
		filename := filepath.Base(file)

		// Check if migration was already applied
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", filename).Scan(&count)
		if err != nil {
			log.Fatal("Failed to check migration status:", err)
		}

		if count > 0 {
			fmt.Printf("Migration %s already applied, skipping\n", filename)
			continue
		}

		// Read and execute migration
		content, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal("Failed to read migration file:", err)
		}

		fmt.Printf("Applying migration %s...\n", filename)
		_, err = db.Exec(string(content))
		if err != nil {
			log.Fatalf("Failed to apply migration %s: %v", filename, err)
		}

		// Mark migration as applied
		_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", filename)
		if err != nil {
			log.Fatal("Failed to record migration:", err)
		}

		fmt.Printf("Successfully applied migration %s\n", filename)
	}

	fmt.Println("All migrations applied successfully!")
}
