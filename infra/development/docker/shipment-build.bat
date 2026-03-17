@echo off
cd services\shipment-service
SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
go build -o ..\..\build\shipment-service .\cmd\main.go
cd ..\..

