# Go POC - Restaurant Card Orders Backend

Small backend to store order items for `Card`s associated with a `Table` for a restaurant.

## Modelagem

A modelagem do domínio (Tabelas, Cartões, Itens e Reservas) foi movida para um arquivo separado em PlantUML.

Veja `docs/modelagem.md` para o diagrama PlantUML e a descrição dos campos e relacionamentos.

## Features
- CRUD for `Table` / `Card` (simple create and get)
- Add/list items for a `Card` (order items)
- SQLite (GORM)

Quick start:

1. Build and run:

```bash
go run ./cmd/server
```

2. Example requests:

Create a table:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"name":"Table 1"}' http://localhost:8080/tables
```

Create a card for a table (use table id from previous step):

```bash
curl -X POST -H "Content-Type: application/json" -d '{"table_id":1, "token":"card-1"}' http://localhost:8080/cards
```

Add an item to a card:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"name":"Coke", "quantity":2, "price":500}' http://localhost:8080/cards/1/items
```

List items for a card:

```bash
curl http://localhost:8080/cards/1/items
```


Notes:
- The app uses a local SQLite database file: `orders.db` in the project root
- Improves: add validations, authentication, full CRUD, and background job
