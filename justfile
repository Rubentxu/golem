# GOLEM — automatización local.
# El gate local es la fuente de verdad para "green" (política CI del
# proyecto); GitHub Actions queda reservado al release gate.

default:
    @just --list

# Formatear todas las fuentes Go
fmt:
    gofmt -w cmd internal adapters tck

# Falla si algún fichero no está gofmt-clean
fmt-check:
    out=$(gofmt -l cmd internal adapters tck); if [ -n "$out" ]; then echo "not gofmt-clean:"; echo "$out"; exit 1; fi

# Ejecutar go vet
vet:
    go vet ./...

# Ejecutar todos los tests (incluye las fitness functions de arquitectura)
test:
    go test ./...

# Compilar todos los binarios
build:
    go build ./...

# Limpiar el módulo
tidy:
    go mod tidy

# Gate local de CI (fuente de verdad para "green")
check: fmt-check vet test
