FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o tag-sync .

FROM scratch
COPY --from=builder /app/tag-sync /tag-sync

EXPOSE 3000
CMD ["/tag-sync"]
