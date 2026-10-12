# Convenções de Módulos de Domínio Go

Patterns extraídos de módulos bem estruturados do projeto. Use como referência ao
avaliar se um módulo segue as convenções arquiteturais esperadas.

> **Escopo:** Estas convenções aplicam-se apenas a módulos que modelam um bounded context ou domínio
> com entidades, use cases e repositórios. Pacotes utilitários, compartilhados, CLIs, de coordenação/orquestração
> pura, e infraestrutura, por exemplo, não são módulos de domínio e não precisam seguir estes patterns.

---

## Estrutura de diretório esperada

Um módulo de domínio Go neste projeto segue esta organização:

```
<modulo>/
├── entity.go              # Entidade de domínio, interfaces, input structs
├── entity_test.go         # Testes da entidade
├── events.go              # Payloads de domain events (pub/sub DTOs)
├── service.go             # Implementação dos use cases
├── service_test.go        # Testes do service
├── mocks/
│   └── mocks.go           # Mocks gerados para interfaces do domínio
├── <storage>/             # Ex: dynamodb/, postgres/
│   ├── interfaces.go      # Interface local para SDK externo
│   ├── repository.go      # Implementação do repositório
│   ├── repository_test.go
│   └── mocks/
│       └── mocks.go       # Mocks do SDK wrapper
└── <integration>/         # Ex: gmud/, notification/
    ├── service.go         # Implementação da integração
    └── service_test.go
```

**Princípio chave:** O pacote raiz (`<modulo>/`) é o domínio. Subpacotes são infraestrutura ou integrações.

---

## Pattern 1: Entidade com comportamento (Rich Domain Model)

A entidade possui métodos que encapsulam invariantes de negócio e transições de estado.

```go
// maintenance/event.go

type Event struct {
    EventID      string
    Status       string
    // ... outros campos
}

// State machine como método da entidade.
func (e *Event) ChangeStatus(newStatus string) error {
    allowed, ok := allowedTransitions[e.Status]
    if !ok || !allowed[newStatus] {
        return errors.New("invalid status transition")
    }
    e.Status = newStatus
    return nil
}

// Query de estado como método da entidade.
func (e *Event) IsTerminal() bool {
    return e.Status == StatusCompleted || e.Status == StatusCancelled
}

// Transições válidas definidas junto da entidade.
var allowedTransitions = map[string]map[string]bool{
    StatusScheduled:  {StatusInProgress: true, StatusCancelled: true},
    StatusInProgress: {StatusCompleted: true, StatusCancelled: true},
}
```

**Porquê:** Invariantes de negócio são propriedade da entidade. Services orquestram, entidades decidem.

**Anti-pattern:** Service contendo regra que pertence à entidade, ou mutação direta de estado:

```go
// ERRADO: regra de transição no service em vez da entidade.
func (s *Service) UpdateStatus(ctx context.Context, id, newStatus string) error {
    event, _ := s.repo.GetByID(ctx, id)
    if event.Status == "completed" && newStatus == "scheduled" {
        return errors.New("cannot go back to scheduled")
    }
    event.Status = newStatus // Mutação direta sem método da entidade.
    return s.repo.Update(ctx, event)
}
```

---

## Pattern 2: Interfaces de use case segregadas no domínio

Interfaces finas, cada uma representando uma capability distinta, definidas no pacote de domínio.

```go
// maintenance/event.go

type EventQueryUseCase interface {
    GetByID(ctx context.Context, id string) (*Event, error)
    FindAllByResourceID(ctx context.Context, resourceID string) ([]*Event, error)
    FindAllByResourceType(ctx context.Context, resourceType string) ([]*Event, error)
}

type EventCreationUseCase interface {
    Create(ctx context.Context, input EventCreationInput) (*Event, error)
}

type EventUpdateUseCase interface {
    Update(ctx context.Context, input EventUpdateInput) (*Event, error)
}

type ExternalSyncUseCase interface {
    SyncExternalEvent(ctx context.Context, input ExternalEventInput) (*Event, error)
}
```

**Porquê:** Consumidores dependem apenas da interface que precisam (ISP). Um handler HTTP que só lê dados depende de `EventQueryUseCase`, não do service inteiro.

**Anti-pattern:** Handler dependendo do service concreto, acoplando a todas as operações:

```go
// ERRADO: handler acoplado ao tipo concreto.
type Handler struct {
    service *maintenance.EventsService
}

// CORRETO: handler depende apenas da interface que precisa.
type Handler struct {
    query  maintenance.EventQueryUseCase
    create maintenance.EventCreationUseCase
}
```

---

## Pattern 3: Interface de repositório no domínio, implementação na infra

```go
// maintenance/event.go (DOMÍNIO)
type EventRepository interface {
    GetByID(ctx context.Context, eventID string) (*Event, error)
    Create(ctx context.Context, event *Event) error
    Update(ctx context.Context, event *Event, expectedUpdatedAt time.Time) error
    // ...
}

// maintenance/dynamodb/repository.go (INFRAESTRUTURA)
var _ maintenance.EventRepository = (*Repository)(nil)

type Repository struct {
    client    DynamoClient
    tableName string
}

func NewRepository(client DynamoClient, tableName string) *Repository {
    return &Repository{client: client, tableName: tableName}
}
```

**Porquê:** O domínio define o contrato (o que precisa). A infra decide como implementar. Dependency Inversion.

---

## Pattern 4: Interface guards em todas as implementações

```go
// Service implementando múltiplas use case interfaces.
var (
    _ EventQueryUseCase    = (*EventsService)(nil)
    _ EventCreationUseCase = (*EventsService)(nil)
    _ EventUpdateUseCase   = (*EventsService)(nil)
    _ ExternalSyncUseCase  = (*EventsService)(nil)
)

// Repositório implementando interface do domínio.
var _ maintenance.EventRepository = (*Repository)(nil)
```

**Porquê:** Falha em tempo de compilação se a struct não satisfaz a interface. Documentação viva do contrato.

---

## Pattern 5: Input structs co-localizadas com a interface

```go
// maintenance/event.go
// Input struct definida logo após a interface que a consome.

type EventCreationUseCase interface {
    Create(ctx context.Context, input EventCreationInput) (*Event, error)
}

type EventCreationInput struct {
    ExternalID   string
    ResourceType string
    ResourceID   string
    EventDate    time.Time
    Description  string
    Source       string
}
```

**Porquê:** Quem lê a interface encontra imediatamente o contrato de dados. Não precisa navegar para outro pacote.

**Anti-pattern:** Input structs em pacote separado (`dto/`, `request/`) ou distantes da interface:

```go
// ERRADO: input struct em pacote separado.
// dto/maintenance_inputs.go
type CreateEventInput struct { ... }

// ERRADO: domain object usado diretamente como DTO entre camadas.
func (h *Handler) Create(ctx context.Context) {
    event := &maintenance.Event{...} // Expõe entidade de domínio na camada HTTP.
    h.service.Create(ctx, event)
}
```

---

## Pattern 6: Domain events como payloads tipados

```go
// maintenance/events.go

type EventCreatedPayload struct {
    EventID      string    `json:"event_id"`
    ExternalID   string    `json:"external_id"`
    ResourceType string    `json:"resource_type"`
    ResourceID   string    `json:"resource_id"`
    EventDate    time.Time `json:"event_date"`
    Status       string    `json:"status"`
}

type EventChangedPayload struct {
    EventID string              `json:"event_id"`
    Changes []EventFieldChanged `json:"changes"`
}

type EventFieldChanged struct {
    Field    string `json:"field"`
    OldValue string `json:"old_value"`
    NewValue string `json:"new_value"`
}
```

**Porquê:** Payloads tipados garantem contrato de comunicação. Evitam `map[string]any` e desserialização frágil.

---

## Pattern 7: Infraestrutura encapsulada com interface local

```go
// maintenance/dynamodb/interfaces.go

// Interface local wrapping o SDK externo.
// Apenas os métodos usados são expostos.
type DynamoClient interface {
    GetItem(ctx context.Context, params *dynamodb.GetItemInput, ...) (*dynamodb.GetItemOutput, error)
    Query(ctx context.Context, params *dynamodb.QueryInput, ...) (*dynamodb.QueryOutput, error)
    PutItem(ctx context.Context, params *dynamodb.PutItemInput, ...) (*dynamodb.PutItemOutput, error)
    UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, ...) (*dynamodb.UpdateItemOutput, error)
}
```

**Porquê:** Testabilidade (mock sem AWS real). Decoupling (trocar SDK sem mudar repositório). ISP (só expõe o que usa).

---

## Pattern 8: Modelo de persistência interno com mapeamento bidirecional

```go
// maintenance/dynamodb/repository.go

// Modelo interno, não exportado fora do pacote.
type dynamoDBEvent struct {
    EventID      string  `dynamodbav:"eventId"`
    ExternalID   *string `dynamodbav:"externalId,omitempty"`
    Status       string  `dynamodbav:"status"`
    // ...
}

// Domínio -> Persistência.
func toDBModel(event *maintenance.Event) dynamoDBEvent { ... }

// Persistência -> Domínio.
func toDomainModel(item dynamoDBEvent) *maintenance.Event { ... }
```

**Porquê:** Isola detalhes de storage (nullable fields, naming conventions) da entidade de domínio. Mudanças no schema do DB não propagam para o domínio.

---

## Pattern 9: Optimistic locking como conceito de domínio

```go
// maintenance/event.go (DOMÍNIO)
// Erro sentinela de domínio para conflito de concorrência.
var ErrConcurrentUpdate = errors.New("concurrent update conflict")

// maintenance/dynamodb/repository.go (INFRA)
// Implementação via condition expression do DynamoDB.
func (r *Repository) Update(ctx context.Context, event *maintenance.Event, expectedUpdatedAt time.Time) error {
    // ...
    condExpr := "updatedAt = :expectedUpdatedAt"
    // ...
    var condErr *types.ConditionalCheckFailedException
    if errors.As(err, &condErr) {
        return maintenance.ErrConcurrentUpdate
    }
}
```

**Porquê:** O conceito (optimistic locking) é de domínio. A implementação (condition expression) é de infra. O service trata `ErrConcurrentUpdate` sem saber como foi implementado.

---

## Pattern 10: Mocks gerados e co-localizados

```go
// maintenance/event.go
//go:generate mockgen -source=event.go -destination=mocks/mocks.go -package=mocks

// maintenance/dynamodb/interfaces.go
//go:generate mockgen -source=interfaces.go -destination=mocks/mocks.go -package=mocks
```

Estrutura:
```
maintenance/
├── mocks/mocks.go          # Mocks das interfaces do domínio
└── dynamodb/
    └── mocks/mocks.go      # Mocks do DynamoClient
```

**Porquê:** `//go:generate` torna a geração reproduzível. Mocks vivem em `<pacote>/mocks/` seguindo convenção do projeto. Commitados no repositório.

---

## Grafo de dependências esperado

```
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ dynamodb/ │ │  gmud/   │ │ handler  │
        │  (infra)  │ │ (integr) │ │  (http)  │
        └─────┬────┘ └─────┬────┘ └─────┬────┘
              │            │            │
              └────────────┼────────────┘
                           │
                           v
                    ┌─────────────┐
                    │   domínio   │
                    │ (maintenance)│
                    └─────────────┘

Setas indicam "depende de" -- todas apontam PARA o domínio.
O domínio NÃO importa nenhum subpacote.
```

---

## Quick-reference: regra -> localização esperada

| Elemento | Pacote esperado | Arquivo esperado |
|----------|----------------|------------------|
| Entidade + campos | `<modulo>/` | `entity.go` ou `<nome>.go` |
| Métodos da entidade | `<modulo>/` | Mesmo arquivo da entidade |
| Constantes de domínio | `<modulo>/` | Mesmo arquivo da entidade |
| Interface do repositório | `<modulo>/` | Mesmo arquivo da entidade |
| Interfaces de use case | `<modulo>/` | Mesmo arquivo da entidade |
| Input structs | `<modulo>/` | Mesmo arquivo da interface que consome |
| Payloads de domain events | `<modulo>/` | `events.go` |
| Service (use case impl) | `<modulo>/` | `service.go` |
| Interface guards | `<modulo>/` e subpacotes | Mesmo arquivo da struct |
| Repositório impl | `<modulo>/<storage>/` | `repository.go` |
| Modelo de persistência | `<modulo>/<storage>/` | `repository.go` (não exportado) |
| Interface de SDK wrapper | `<modulo>/<storage>/` | `interfaces.go` |
| Integração (ex: Jira) | `<modulo>/<integracao>/` | `service.go` |
| Mocks do domínio | `<modulo>/mocks/` | `mocks.go` (gerado) |
| Mocks de infra | `<modulo>/<storage>/mocks/` | `mocks.go` (gerado) |
