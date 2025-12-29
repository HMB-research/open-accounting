// Atlas Configuration for Open Accounting
// Manages database schema migrations

variable "db_url" {
  type    = string
  default = getenv("DATABASE_URL")
}

env "local" {
  src = "file://schema.sql"
  url = "postgres://openaccounting:openaccounting@localhost:5432/openaccounting?sslmode=disable"
  dev = "docker://postgres/16/dev?search_path=public"

  migration {
    dir = "file://migrations"
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "docker" {
  src = "file://schema.sql"
  url = "postgres://openaccounting:openaccounting@db:5432/openaccounting?sslmode=disable"
  dev = "docker://postgres/16/dev?search_path=public"

  migration {
    dir = "file://migrations"
  }
}

env "prod" {
  src = "file://schema.sql"
  url = var.db_url

  migration {
    dir = "file://migrations"
  }
}

// Lint rules for migrations
lint {
  destructive {
    error = true
  }
  data_depend {
    error = true
  }
}
