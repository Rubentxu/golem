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

# Verificar que los sha256 de MANIFEST.json coinciden con los ficheros
manifest-check:
    #!/usr/bin/env python3
    import hashlib, json, os, sys
    d = 'golem-documentation'
    with open(os.path.join(d, 'MANIFEST.json')) as f:
        m = json.load(f)
    bad = []
    for path, want in m.get('sha256', {}).items():
        full = os.path.join(d, path)
        if not os.path.exists(full):
            bad.append(f'MISSING: {path}')
            continue
        h = hashlib.sha256(open(full, 'rb').read()).hexdigest()
        if h != want:
            bad.append(f'MISMATCH {path}: manifest={want[:16]}... actual={h[:16]}...')
    if bad:
        for b in bad:
            print(b)
        print(f'FAIL ({len(bad)} mismatch(es))')
        sys.exit(1)
    print(f'OK ({len(m["sha256"])} files verified)')
