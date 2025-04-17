# ===== STAGE 1: Build stage =====
FROM golang:1.23-alpine AS builder

# ติดตั้ง dependencies ที่จำเป็น
RUN apk add --no-cache git

# ตั้งค่า Working Directory
WORKDIR /app

# คัดลอก go.mod และ go.sum เพื่อให้ Docker cache ได้
COPY go.mod go.sum ./

# ดาวน์โหลด dependencies
RUN go mod download

# คัดลอก source code ทั้งหมด
COPY . .

# คอมไพล์โค้ดเป็นไบนารี
RUN CGO_ENABLED=0 go build -o main cmd/main.go

# ===== STAGE 2: Run stage (ใช้ Alpine เพื่อลดขนาด image) =====
FROM alpine:latest

# ตั้งค่า Working Directory
WORKDIR /root/

# คัดลอก binary ที่ build เสร็จแล้วมาจาก builder stage
COPY --from=builder /app/main .

# ตรวจสอบว่าต้องใช้ไฟล์ static หรือไม่ และคัดลอกอย่างเหมาะสม
COPY --from=builder /app/cmd/docs/ cmd/docs/
COPY --from=builder /app/internal/database/sql/ internal/database/sql/

# ตั้งค่าให้ binary เป็น executable
RUN chmod +x ./main

# เปิดพอร์ต 8080
EXPOSE 8080

# ใช้ ENTRYPOINT แทน CMD
ENTRYPOINT ["./main"]