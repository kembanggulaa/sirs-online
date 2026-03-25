# SIRS Online Bridging System V3

An automated system to synchronize hospital bed availability data from **SIMRS (SQL Server)** to the **Kemenkes RS Online API**.

## 🚀 Overview

This application serves as a bridge for RSUD Sleman to automate the reporting of bed availability. It includes a background worker pool for scheduled synchronization and a web-based dashboard for real-time monitoring and manual data management.

## ✨ Key Features

- **Automated Synchronization**: Runs every 2 hours (configurable) using a high-performance worker pool.
- **Transactional Database Layer**: Ensures data integrity by handling temporary tables within SQL Server transactions.
- **Web Dashboard**: Modern interface built with **Tailwind CSS** and **Alpine.js** for monitoring sync status, viewing logs, and manual intervention.
- **Windows Service Support**: Can be deployed as a persistent service on Windows Servers.
- **Security**: Automated header generation (`X-Timestamp`, etc.) for Kemenkes API compliance.

## 🛠 Tech Stack

- **Backend**: Golang 1.23+
- **Database**: Microsoft SQL Server
- **Frontend**: Alpine.js, Tailwind CSS
- **Configuration**: Viper (.env)
- **HTTP Client**: Resty

## 📖 Documentation

Detailed documentation and technical specifications are available in Indonesian:
- [Detailed Guide (DOKUMENTASI.md)](DOKUMENTASI.md)
- [Architecture & Task List (architecture.md)](architecture.md)

## 🚦 Quick Start

### 1. Prerequisites
- [Go 1.23+](https://golang.org/dl/)
- Access to SIMRS SQL Server database.
- Kemenkes RS Online API credentials.

### 2. Installation
Clone the repository and install dependencies:
```bash
go mod tidy
```

### 3. Configuration
Create a `.env` file in the root directory (refer to [DOKUMENTASI.md](DOKUMENTASI.md) for placeholders).

### 4. Running the App
```bash
# Development mode
go run main.go
```

## 📝 License
Proprietary - RSUD Sleman.
