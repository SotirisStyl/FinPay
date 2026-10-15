# FinPay

**A Go-based financial backend API for customer and loan management.**

FinPay is a backend engineering project built with Go and PostgreSQL. It focuses on REST API design, database modeling, input validation, error handling, automated testing, and maintainable backend architecture.

The project is being developed incrementally, with features implemented through focused tasks, feature branches, pull requests, and code reviews.

## Features

### Customer management
- Create customers with input validation.
- Retrieve customers by UUID.
- Partially update customer details.
- Soft-delete customers.
- Handle duplicate email addresses and invalid requests.
- Return appropriate HTTP status codes for common error scenarios.

### Loan management
- Model loans and their relationship to customers.
- Retrieve loans belonging to a customer.
- Use PostgreSQL decimal storage for monetary amounts.
- Exclude loans associated with soft-deleted customers where applicable.

## Tech Stack

- **Language:** Go
- **HTTP framework:** Echo
- **Database:** PostgreSQL
- **ORM:** GORM
- **Testing:** Go testing, `sqlmock`, and HTTP handler tests
- **Database migrations:** SQL migration files
- **Version control:** Git and GitHub pull requests

## API Overview

The API is mounted under `/api`.

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/customers` | Create a customer |
| `GET` | `/api/customers/:id` | Retrieve a customer |
| `PATCH` | `/api/customers/:id` | Partially update a customer |
| `DELETE` | `/api/customers/:id` | Soft-delete a customer |
| `GET` | `/api/customers/:id/loans` | Retrieve a customer's loans |

Customer IDs are UUIDs. The API validates incoming requests and distinguishes client errors, missing records, and unexpected server errors.

## Engineering Practices

This project emphasizes:

- Clear separation of HTTP handlers, models, and database-related logic.
- Input validation and consistent error responses.
- Automated tests for success cases and failure scenarios.
- SQL migrations for database schema changes.
- Explicit handling of soft deletion and customer–loan relationships.
- Small, reviewable pull requests.

## Getting Started

### Prerequisites

- Go
- PostgreSQL
- Git

### Clone the repository

```bash
git clone https://github.com/SotirisStyl/FinPay.git
cd FinPay
```

### Configure the database

Create a local PostgreSQL database and configure the environment variables expected by the application.

Check the configuration code and migration setup in the repository for the exact variable names and database requirements before starting the server.

### Run the application

```bash
go run .
```

The application defaults to port `8080`, unless overridden by the `PORT` environment variable.

### Run the tests

```bash
go test ./...
```

## Project Status

FinPay is an active learning and portfolio project. Features and architecture will evolve as additional backend engineering capabilities are implemented.

The repository documents development through feature branches, pull requests, and iterative code review.

## Future Direction

Potential areas of continued development include loan creation, stronger API documentation, database integration testing, CI automation, and additional backend infrastructure.

---

**Author:** [SotirisStyl](https://github.com/SotirisStyl)
