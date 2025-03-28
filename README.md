### Prepare
#### Instal Dapr CLI
Windows (Powershell)
```bash
iwr -useb https://raw.githubusercontent.com/dapr/cli/master/install/install.ps1 | iex
```

MacOS
```bash
brew install dapr/tap/dapr-cli
```

Linux
```bash
wget -q https://raw.githubusercontent.com/dapr/cli/master/install/install.sh -O - | /bin/bash
```

### Inisialisasi Dapr
```bash
dapr init
```


### Run Customer Service
```bash
dapr run --app-id customer-service --app-port 8081 --dapr-http-port 3501 -- go run customer-service/main.go
```

### Run Product Service
```bash
dapr run --app-id product-service --app-port 8082 --dapr-http-port 3502 -- go run product-service/main.go
```

### Run Transaction Service
```bash
dapr run --app-id transaction-service --app-port 8083 --dapr-http-port 3503 -- go run transaction-service/main.go
```

### Dapr Dekstop
```bash
dapr dashboard
```

### Run Zipkin
Zipkin adalah sistem pelacakan terdistribusi sumber 
terbuka yang dirancang untuk membantu memantau dan 
memecahkan masalah latensi dalam arsitektur layanan mikro. 
Dengan Zipkin, Anda dapat mengumpulkan data waktu dari 
berbagai layanan dalam aplikasi Anda, memungkinkan identifikasi 
titik-titik bottleneck dan pemahaman alur permintaan antar layanan. 
```bash
dapr run --app-id zipkin --app-port 9411 --dapr-http-port 3504
```