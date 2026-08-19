# Golem Infrastructure — Quadlets
#
# Podman quadlets (systemd unit files for containers).
# Gestionados por systemd, no por `podman run`.
#
# ## Estructura
#
#   infra/quadlets/
#   ├── nats.container          — NATS JetStream (dev transport + benchmark)
#   ├── hugegraph.container     — Apache HugeGraph (candidate #1)
#   ├── dgraph.container       — Dgraph (candidate #3)
#   └── nebula-allinone.container — NebulaGraph all-in-one (candidate #2)
#
# ## Instalación
#
# 1. Crear el directorio de quadlets para el usuario:
#    mkdir -p ~/.config/containers/systemd/
#
# 2. Symlink los quadlets:
#    ln -s /var/home/rubentxu/Proyectos/golang/golem/infra/quadlets/*.container \
#       ~/.config/containers/systemd/
#
# 3. Recargar systemd:
#    systemctl --user daemon-reload
#
# 4. Levantar:
#    systemctl --user start nats
#    systemctl --user start hugegraph
#    systemctl --user start dgraph
#    systemctl --user start nebula-allinone
#
# 5. Verificar:
#    systemctl --user status nats hugegraph dgraph nebula-allinone
#
# 6. Logs:
#    journalctl --user -u nats -f
#    journalctl --user -u hugegraph -f
#    journalctl --user -u dgraph -f
#    journalctl --user -u nebula-allinone -f
#
# ## Health checks
#
# Todos los quadlets tienen HealthCmd configurado.
# systemd-healthcheck.service monitors them automatically.
#
# Verificar health:
#    podman healthcheck run golem-nats
#    podman healthcheck run golem-hugegraph
#    podman healthcheck run golem-dgraph
#    podman healthcheck run golem-nebula-allinone
#
# ## Dependencias entre quadlets
#
# NebulaGraph necesita que metad y storaged estén corriendo antes que graphd.
# El entrypoint de nebula-allinone hace el start secuencial (sleep 3 entre cada).
#
# HugeGraph puede tardar ~20s en estar listo después del primer inicio.
# Dgraph está listo en ~5s.
# NATS está listo en ~2s.
#
# ## Limpieza
#
#    systemctl --user stop nats hugegraph dgraph nebula-allinone
#    podman volume rm golem-nats-data golem-hugegraph-data \
#       golem-nebula-data golem-dgraph-data
#
# ## Rebuild de imágenes locales
#
# Si modificas el Containerfile de algún servicio:
#    cd infra
#    podman build -t golem-nats:latest ./nats
#    podman build -t golem-hugegraph:latest ./hugegraph
#    podman build -t golem-dgraph:latest ./dgraph
#
# Luego reiniciar el quadlet:
#    systemctl --user restart <service>
