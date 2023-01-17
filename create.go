package goose

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type tmplVars struct {
	Version   string
	CamelName string
}

var (
	sequential = false
	table      = ""
)

// SetSequential set whether to use sequential versioning instead of timestamp based versioning
func SetSequential(s bool) {
	sequential = s
}

// Create writes a new blank migration file.
func CreateWithTemplate(db *sql.DB, dir string, tmpl *template.Template, name, migrationType string) error {
	var version string
	if sequential {
		// always use DirFS here because it's modifying operation
		migrations, err := collectMigrationsFS(osFS{}, dir, minVersion, maxVersion)
		if err != nil {
			return err
		}

		vMigrations, err := migrations.versioned()
		if err != nil {
			return err
		}

		if last, err := vMigrations.Last(); err == nil {
			version = fmt.Sprintf(seqVersionTemplate, last.Version+1)
		} else {
			version = fmt.Sprintf(seqVersionTemplate, int64(1))
		}
	} else {
		version = time.Now().Format(timestampFormat)
	}

	snakeCaseName := snakeCase(name)
	filename := fmt.Sprintf("%v_%v.%v", version, snakeCaseName, migrationType)

	if tmpl == nil {
		if migrationType == "go" {
			tmpl = goSQLMigrationTemplate
		} else {
			var commandType string
			tmpl = sqlMigrationTemplate
			splited := strings.Split(snakeCaseName, "_")
			commandType = splited[0]
			table = splited[1]
			var sqlCreateMigrationTemplate = template.Must(
				template.New("goose.sql-migration").Parse(
					fmt.Sprintf(`-- +goose Up
-- +goose StatementBegin
CREATE TABLE %v (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT ,
    created_at TIMESTAMP NULL ,
    updated_at TIMESTAMP NULL , PRIMARY KEY (id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE %v;
-- +goose StatementEnd
`, table, table)))

			var sqlUpdateMigrationTemplate = template.Must(
				template.New("goose.sql-migration").Parse(
					fmt.Sprintf(`-- +goose Up
-- +goose StatementBegin
ALTER TABLE %v;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE %v;
-- +goose StatementEnd
`, table, table)))
			switch commandType {
			case "create":
				tmpl = sqlCreateMigrationTemplate
			case "update":
				tmpl = sqlUpdateMigrationTemplate
			}
		}
	}

	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("failed to create migration file: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}
	defer f.Close()

	vars := tmplVars{
		Version:   version,
		CamelName: camelCase(name),
	}
	if err := tmpl.Execute(f, vars); err != nil {
		return fmt.Errorf("failed to execute tmpl: %w", err)
	}

	log.Printf("Created new file: %s\n", f.Name())
	return nil
}

// Create writes a new blank migration file.
func Create(db *sql.DB, dir, name, migrationType string) error {
	return CreateWithTemplate(db, dir, nil, name, migrationType)
}

var sqlMigrationTemplate = template.Must(template.New("goose.sql-migration").Parse(`-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
`))

var goSQLMigrationTemplate = template.Must(template.New("goose.go-migration").Parse(`package migrations

import (
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(up{{.CamelName}}, down{{.CamelName}})
}

func up{{.CamelName}}(tx *sql.Tx) error {
	// This code is executed when the migration is applied.
	return nil
}

func down{{.CamelName}}(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
`))
