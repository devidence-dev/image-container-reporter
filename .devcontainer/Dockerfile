FROM debian:13.1-slim

# Instalar dependencias del sistema
RUN apt-get update && apt-get install -y \
    curl \
    git \
    jq \
    ca-certificates \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Crear usuario vscode
RUN useradd -m -s /bin/bash vscode && \
    echo "vscode ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Instalar Go 1.25.2
RUN curl -fsSL https://go.dev/dl/go1.25.2.linux-$(dpkg --print-architecture).tar.gz | \
    tar -C /usr/local -xzf -

# Configurar PATH para Go
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/home/vscode/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Cambiar a usuario vscode
USER vscode
WORKDIR /home/vscode

# Instalar herramientas de desarrollo Go
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest

# Configurar directorio de trabajo
WORKDIR /workspace