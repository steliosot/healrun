FROM golang:1.21 AS build

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/healrun ./cmd/healrun

FROM ubuntu:22.04

# Install healrun binary built for Linux
COPY --from=build /out/healrun /usr/local/bin/healrun
RUN chmod +x /usr/local/bin/healrun

# 1️⃣ Missing apt update
RUN healrun "apt-get install -y curl"

# 2️⃣ Missing Python
RUN healrun "python3 --version"

# 3️⃣ Missing pip
RUN healrun "pip3 install requests"

# 4️⃣ Missing build tools (C compiler)
RUN healrun "pip3 install psycopg2"

# 5️⃣ Node native module build
RUN healrun "apt-get install -y nodejs npm"
RUN healrun "npm install bcrypt"

# 6️⃣ Missing CLI tool
RUN healrun "wget https://example.com"

# 7️⃣ Another missing system lib
RUN healrun "pip3 install lxml"

CMD ["echo", "Build completed"]
