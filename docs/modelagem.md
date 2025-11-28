# Modelagem (PlantUML)

Este arquivo contém o diagrama de classes do domínio em PlantUML.

Para renderizar localmente, você pode usar uma extensão PlantUML no seu editor ou o site oficial do PlantUML.

```plantuml
@startuml
' Modelagem de domínio — Restaurante: Table, Card, Item, Reservation

class Table {
  +uint ID
  +string Name
  +string Status
  +time CreatedAt
  +time UpdatedAt
}

class Card {
  +uint ID
  +uint TableID
  +string Token
  +time CreatedAt
  +time UpdatedAt
}

class Item {
  +uint ID
  +uint CardID
  +uint MenuItemID
  +string Name
  +int Quantity
  +int PriceCents
  +string Status
  +time CreatedAt
  +time UpdatedAt
}

class Reservation {
  +uint ID
  +uint TableID
  +time StartAt
  +time EndAt
  +string Status
  +time CreatedAt
  +time UpdatedAt
}

class Menu {
  +uint ID
  +string Name
  +time CreatedAt
  +time UpdatedAt
}

class MenuItem {
  +uint ID
  +uint MenuID
  +string Name
  +string Description
  +int PriceCents
  +bool Available
  +time CreatedAt
  +time UpdatedAt
}

Table "1" -- "0..*" Card : has
Card "1" -- "0..*" Item : has
Table "1" -- "0..*" Reservation : reservations
Menu "1" -- "0..*" MenuItem : contains
MenuItem "1" -- "0..*" Item : can_be_ordered_as

note right of Table
  Status: free | reserved | in_use
end note

note right of Reservation
  Status: pending | confirmed | started | completed | cancelled
end note

@enduml
```

Descrição rápida:
- `Table` representa uma mesa do restaurante.
- `Card` (cartão) é o pedido associado a uma `Table`.
- `Item` são os itens pedidos no `Card`.
- `Reservation` representa uma reserva de mesa com início/fim (calendarizada).

Menu e MenuItem:
- `Menu` agrupa os itens disponíveis (ex: "Almoço", "Bebidas").
- `MenuItem` representa um item do cardápio com preço e disponibilidade.

Relação com pedidos:
- `Item` pode referenciar um `MenuItem` via `MenuItemID` (opcional). Isso permite registrar o nome/price no momento do pedido e manter histórico mesmo se o cardápio mudar.

API relevante:
- `POST /reservations` — criar reserva
- `POST /reservations/{id}/checkin` — check-in (iniciar reserva)
- `POST /reservations/{id}/checkout` — check-out (finalizar reserva)
- `GET /tables/{id}/reservations` — listar reservas da mesa
