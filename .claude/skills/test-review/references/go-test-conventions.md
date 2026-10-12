# Convenções de Testes Go

Patterns e anti-patterns extraídos do projeto e de boas práticas da comunidade Go.
Use como referência ao avaliar qualidade de testes.

> **Escopo:** Estas convenções aplicam-se a todos os arquivos `_test.go` do projeto,
> independentemente da camada (domínio, infraestrutura, integrações, handlers).

---

## Estrutura de teste esperada

Um pacote Go neste projeto segue esta organização de testes:

```
<package>/
├── foo.go              # Código de produção
├── foo_test.go         # Testes correspondentes
├── bar.go
├── bar_test.go
└── mocks/
    └── mocks.go        # Mocks gerados (uber-go/mock)
```

**Princípio chave:** Todo arquivo `.go` com lógica pública deve ter um `_test.go` correspondente.

---

## Pattern 1: Table-driven tests

Padrão principal do projeto. Cada cenário é definido como uma entrada na tabela, promovendo reuso e clareza. Os campos da struct (`input`, `setupMock`, `wantErr`) servem naturalmente como a fase de setup — comentários AAA são desnecessários.

```go
func TestEventsService_Create(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        input     maintenance.EventCreationInput
        setupMock func(repo *mocks.MockEventRepository)
        wantErr   bool
        wantEvent *maintenance.Event
    }{
        "creates event with valid input": {
            input: maintenance.EventCreationInput{
                ExternalID:   "ext-123",
                ResourceType: "cluster",
            },
            setupMock: func(repo *mocks.MockEventRepository) {
                repo.EXPECT().
                    Create(gomock.Any(), gomock.Any()).
                    Return(nil)
            },
            wantErr: false,
        },
        "returns error when repository fails": {
            input: maintenance.EventCreationInput{
                ExternalID: "ext-123",
            },
            setupMock: func(repo *mocks.MockEventRepository) {
                repo.EXPECT().
                    Create(gomock.Any(), gomock.Any()).
                    Return(errors.New("db error"))
            },
            wantErr: true,
        },
    }

    for name, tc := range tests {
        t.Run(name, func(t *testing.T) {
            t.Parallel()

            ctrl := gomock.NewController(t)
            repo := mocks.NewMockEventRepository(ctrl)
            tc.setupMock(repo)
            svc := maintenance.NewEventsService(repo)

            result, err := svc.Create(t.Context(), tc.input)

            if tc.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.NotNil(t, result)
        })
    }
}
```

**Porquê:** Facilita adicionar novos cenários sem duplicar boilerplate. Cada cenário é autocontido e nomeado.

**Anti-pattern:** Testes sequenciais sem subtestes ou com lógica duplicada:

```go
// ERRADO: sem table-driven, lógica duplicada.
func TestCreate(t *testing.T) {
    ctrl := gomock.NewController(t)
    repo := mocks.NewMockEventRepository(ctrl)
    repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
    svc := maintenance.NewEventsService(repo)
    result, err := svc.Create(context.Background(), input)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}

func TestCreateError(t *testing.T) {
    // ... mesma estrutura duplicada ...
}
```

---

## Pattern 1b: Fallback AAA para testes não table-driven

Quando table-driven não é viável ou benéfico, usar estrutura AAA (Arrange, Act, Assert) explícita com comentários.

```go
func TestCatalogService_SyncWithExternalProvider(t *testing.T) {
    t.Parallel()

    // Arrange
    ctrl := gomock.NewController(t)
    provider := mocks.NewMockExternalProvider(ctrl)
    repo := mocks.NewMockCatalogRepository(ctrl)

    existingItems := []*catalog.Item{
        {ID: "item-1", Version: 1},
        {ID: "item-2", Version: 3},
    }
    repo.EXPECT().ListAll(gomock.Any()).Return(existingItems, nil)
    provider.EXPECT().FetchUpdates(gomock.Any(), existingItems).Return([]*catalog.Update{
        {ItemID: "item-1", NewVersion: 2},
    }, nil)
    repo.EXPECT().ApplyUpdates(gomock.Any(), gomock.AssignableToTypeOf([]*catalog.Update{})).Return(nil)

    svc := catalog.NewService(repo, provider)

    // Act
    result, err := svc.Sync(t.Context())

    // Assert
    require.NoError(t, err)
    assert.Equal(t, 1, result.UpdatedCount)
    assert.Equal(t, 1, result.SkippedCount)
}
```

**Quando usar AAA em vez de table-driven:**
- Cenário único complexo que não se beneficia de tabela (sem variações de input).
- Setup com múltiplas dependências encadeadas difíceis de expressar em `setupMock` func.
- Testes de integração com lifecycle complexo (setup DB, seed data, execute, verify, cleanup).

**Anti-pattern:** Teste não table-driven com setup complexo sem separação clara:

```go
// ERRADO: setup, execução e verificação entrelaçados.
func TestService_ComplexFlow(t *testing.T) {
    repo := setupRepo(t)
    items, err := repo.List(t.Context())
    assert.NoError(t, err) // Assert no meio do setup?
    svc := NewService(repo)
    svc.Process(t.Context(), items)
    result, err := svc.GetResult(t.Context())
    assert.NoError(t, err)
    assert.Len(t, result.Processed, len(items)) // Onde termina o Act?
}
```

---

## Pattern 2: Nomenclatura de testes

### Nível superior

Formato: `Test<Type>_<Method>` para métodos, `Test<Function>` para funções.

```go
func TestEventsService_Create(t *testing.T) { ... }
func TestEventsService_Update(t *testing.T) { ... }
func TestNewEventsService(t *testing.T) { ... }
```

### Subtestes

Nomes descritivos em inglês, legíveis como frases. Espaços são aceitos (Go os converte em underscores).

```go
// BOM: descritivos, lêem como comportamento.
t.Run("creates event with valid input", ...)
t.Run("returns error when repository fails", ...)
t.Run("rejects invalid status transition", ...)

// RUIM: vagos ou técnicos demais.
t.Run("test1", ...)
t.Run("error case", ...)
t.Run("TestCreateWithInvalidInputReturnsError", ...) // PascalCase em subteste
```

**Convenção do projeto:** Use consistentemente `tc` ou `tt` como variável de iteração, mas não misture ambos no mesmo arquivo.

---

## Pattern 3: Mocks específicos com uber-go/mock

Expectativas devem ser específicas sobre argumentos e número de chamadas.

```go
// BOM: expectativa específica.
repo.EXPECT().
    GetByID(gomock.Any(), "event-123").
    Return(&maintenance.Event{EventID: "event-123"}, nil).
    Times(1)

// RUIM: gomock.Any() para tudo.
repo.EXPECT().
    GetByID(gomock.Any(), gomock.Any()).
    Return(&maintenance.Event{}, nil).
    AnyTimes()
```

**Porquê:** Mocks genéricos mascaram bugs. Se o código passa `"event-456"` quando deveria passar `"event-123"`, o teste com `gomock.Any()` não detecta.

**Quando `gomock.Any()` é aceitável:**
- Para `context.Context` como primeiro argumento (padrão do projeto).
- Para argumentos gerados dinamicamente (UUIDs, timestamps) que são verificados de outra forma.

**Anti-pattern:** `.AnyTimes()` em expectativas de operações de escrita:

```go
// ERRADO: não valida quantas vezes o repositório é chamado.
repo.EXPECT().
    Create(gomock.Any(), gomock.Any()).
    Return(nil).
    AnyTimes()
```

---

## Pattern 4: Contexto e lifecycle

### Usar `t.Context()`

```go
// BOM: propaga cancelamento do teste.
result, err := svc.Create(t.Context(), input)

// RUIM: contexto desconectado do teste.
result, err := svc.Create(context.Background(), input)
```

### `t.Helper()` em funções auxiliares

```go
// BOM: marca como helper para stack traces limpos.
func newTestEvent(t *testing.T) *maintenance.Event {
    t.Helper()
    return &maintenance.Event{
        EventID: "test-" + uuid.New().String(),
        Status:  maintenance.StatusScheduled,
    }
}
```

### Controller do gomock

```go
// BOM: gomock com t — cleanup automático.
ctrl := gomock.NewController(t)

// DESNECESSÁRIO: defer ctrl.Finish() é automático quando t é passado.
ctrl := gomock.NewController(t)
defer ctrl.Finish() // redundante
```

---

## Pattern 5: Paralelismo em testes

```go
func TestEventsService_Create(t *testing.T) {
    t.Parallel() // Nível superior

    tests := map[string]struct{ ... }{...}

    for name, tc := range tests {
        t.Run(name, func(t *testing.T) {
            t.Parallel() // Cada subteste também
            // ... test body ...
        })
    }
}
```

**Porquê:** Testes paralelos executam mais rápido e expõem race conditions escondidas.

**Anti-pattern:** Compartilhar estado mutável entre subtestes:

```go
// ERRADO: variável compartilhada entre subtestes paralelos.
counter := 0
for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        counter++ // DATA RACE
    })
}
```

---

## Pattern 6: Setup de mocks via função no test case

Delegar a configuração de mocks a uma função no próprio test case promove isolamento.

```go
tests := map[string]struct {
    setupMock func(repo *mocks.MockEventRepository, pub *mocks.MockPublisher)
    wantErr   bool
}{
    "publishes event after creation": {
        setupMock: func(repo *mocks.MockEventRepository, pub *mocks.MockPublisher) {
            repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
            pub.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)
        },
        wantErr: false,
    },
}
```

**Anti-pattern:** Setup de mocks compartilhado fora do loop com `.AnyTimes()`:

```go
// ERRADO: setup compartilhado mascara dependências entre cenários.
repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(event, nil).AnyTimes()
for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        // todos os subtestes compartilham a mesma expectativa
    })
}
```

---

## Pattern 7: Cobertura de cenários

Para cada método público, testar ao menos:

| Cenário | Tipo |
|---------|------|
| Input válido, caminho feliz | Happy path |
| Input inválido (campos obrigatórios ausentes) | Validação |
| Dependência retorna erro (repositório, cliente externo) | Error propagation |
| Edge cases (lista vazia, nil, valores limite) | Boundary |
| Transições de estado inválidas (se aplicável) | State machine |
| Erros de concorrência (se aplicável) | Conflict |

**Anti-pattern:** Testar apenas o happy path:

```go
// INSUFICIENTE: só testa sucesso.
func TestService_Create(t *testing.T) {
    // ... setup mock to return nil ...
    result, err := svc.Create(t.Context(), validInput)
    assert.NoError(t, err)
    assert.NotNil(t, result)
    // Onde está o teste de erro do repositório?
    // Onde está o teste de input inválido?
}
```

---

## Pattern 8: Resistência a regressão

Testes devem validar **comportamento observável**, não **detalhes de implementação**.

```go
// BOM: testa o resultado e efeitos colaterais observáveis.
result, err := svc.Create(t.Context(), input)
assert.NoError(t, err)
assert.Equal(t, "scheduled", result.Status)
assert.NotEmpty(t, result.EventID)

// RUIM: testa ordem de chamadas internas (acoplado à implementação).
gomock.InOrder(
    repo.EXPECT().Validate(gomock.Any()),
    repo.EXPECT().Create(gomock.Any(), gomock.Any()),
    pub.EXPECT().Publish(gomock.Any(), gomock.Any()),
)
```

**Porquê:** Se a implementação muda a ordem interna sem alterar o resultado, testes acoplados quebram sem motivo real.

**Exceção:** `gomock.InOrder` é válido quando a ordem é um requisito de negócio (ex: persistir antes de publicar evento).

---

## Pattern 9: Consistência de framework de mocks

O projeto usa `uber-go/mock` como framework padrão via `//go:generate mockgen`.

```go
// PADRÃO: usar uber-go/mock para todas as camadas.
//go:generate mockgen -source=event.go -destination=mocks/mocks.go -package=mocks
ctrl := gomock.NewController(t)
repo := mocks.NewMockEventRepository(ctrl)

// EVITAR em novos pacotes: testify/mock quando uber-go/mock é o padrão.
type mockRepository struct {
    mock.Mock
}
```

**Porquê:** Consistência reduz carga cognitiva. Um único framework para gerar, configurar e verificar mocks.

**Quando usar `testify/mock`:** Para cenários simples — interfaces com poucos métodos, mocks inline definidos junto ao teste, ou quando a cerimônia de `gomock`/`mockgen` (generate, controller, cleanup) não se justifica. Pacotes existentes que já usam `testify/mock` de forma consistente devem manter o mesmo framework para evitar mistura.

**Quando usar `uber-go/mock`:** Para interfaces complexas, mocks reutilizados entre múltiplos testes, ou quando se precisa de verificação estrita de ordem de chamadas e argumentos tipados.

**Regra geral:** Não misture frameworks de mock dentro do mesmo pacote. Escolha um e mantenha consistência.

---

## Pattern 10: Assertions com testify

Usar `assert` para verificações não-fatais e `require` para pré-condições.

```go
// BOM: require para pré-condição, assert para o resto.
require.NoError(t, err) // Se falhar, o teste para aqui.
assert.Equal(t, expected.ID, result.ID)
assert.Equal(t, expected.Status, result.Status)

// RUIM: assert em pré-condição que causa nil panic depois.
assert.NoError(t, err)       // Continua mesmo se err != nil.
assert.Equal(t, expected.ID, result.ID) // Panic se result é nil.
```

**Porquê:** `require` interrompe o teste na falha, evitando cascatas de erros confusos.

---

## Pattern 11: Non-determinism e flakiness

Testes devem produzir o mesmo resultado independentemente de quando ou onde executam.

```go
// RUIM: depende do relógio real — pode falhar em horários específicos.
func TestEvent_IsExpired(t *testing.T) {
    event := &Event{ExpiresAt: time.Now().Add(-1 * time.Hour)}
    assert.True(t, event.IsExpired()) // E se time.Now() muda entre construção e check?
}

// BOM: clock injetado, resultado determinístico.
func TestEvent_IsExpired(t *testing.T) {
    t.Parallel()
    fixedNow := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
    event := &Event{ExpiresAt: fixedNow.Add(-1 * time.Hour)}
    assert.True(t, event.IsExpired(fixedNow))
}
```

```go
// RUIM: time.Sleep para sincronização — flaky por timing.
func TestPublisher_Publish(t *testing.T) {
    pub.Publish(t.Context(), msg)
    time.Sleep(100 * time.Millisecond) // Esperando o async terminar?
    assert.Equal(t, 1, subscriber.Count())
}

// BOM: sincronização explícita.
func TestPublisher_Publish(t *testing.T) {
    t.Parallel()
    done := make(chan struct{})
    subscriber.OnReceive(func() { close(done) })
    pub.Publish(t.Context(), msg)
    select {
    case <-done:
    case <-time.After(5 * time.Second):
        t.Fatal("timeout waiting for message")
    }
    assert.Equal(t, 1, subscriber.Count())
}
```

```go
// RUIM: chamada HTTP real — depende de serviço externo.
func TestClient_Fetch(t *testing.T) {
    client := NewClient("https://api.external.com")
    result, err := client.Fetch(t.Context(), "resource-1")
    assert.NoError(t, err)
}

// BOM: httptest server para controle total.
func TestClient_Fetch(t *testing.T) {
    t.Parallel()
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"id": "resource-1"})
    }))
    t.Cleanup(srv.Close)
    client := NewClient(srv.URL)
    result, err := client.Fetch(t.Context(), "resource-1")
    assert.NoError(t, err)
    assert.Equal(t, "resource-1", result.ID)
}
```

**Porquê:** Testes flaky corroem a confiança na suite inteira. Se o time começa a ignorar falhas "porque é flaky", regressões reais passam despercebidas.

---

## Pattern 12: Eficiência de nível de teste

Usar o nível de teste adequado para o que está sendo verificado.

```go
// RUIM: setup pesado de integração para testar lógica pura.
func TestService_CalculateDiscount(t *testing.T) {
    db := setupTestDatabase(t)           // Subiu um DB real
    repo := postgres.NewRepository(db)   // Repositório real
    svc := NewService(repo)
    discount := svc.CalculateDiscount(100, "VIP")
    assert.Equal(t, 20.0, discount)
    // A lógica de desconto é pura — não precisa de DB.
}

// BOM: teste unitário com mock para lógica pura.
func TestService_CalculateDiscount(t *testing.T) {
    t.Parallel()
    discount := CalculateDiscount(100, "VIP")
    assert.Equal(t, 20.0, discount)
}
```

```go
// RUIM: testando comportamento da lib, não do projeto.
func TestJSONMarshal(t *testing.T) {
    data := map[string]string{"key": "value"}
    bytes, err := json.Marshal(data)
    assert.NoError(t, err)
    assert.Contains(t, string(bytes), "key")
    // Isso testa encoding/json, não o seu código.
}
```

```go
// RUIM: cobertura redundante sem boundary distinction.
tests := map[string]struct{...}{
    "input abc returns success": {input: "abc", wantErr: false},
    "input def returns success": {input: "def", wantErr: false},
    "input ghi returns success": {input: "ghi", wantErr: false},
    // 3 cenários idênticos em comportamento — nenhum testa um boundary.
}

// BOM: cada cenário testa um boundary distinto.
tests := map[string]struct{...}{
    "valid input returns success":  {input: "abc", wantErr: false},
    "empty input returns error":    {input: "",    wantErr: true},
    "input exceeding max length":   {input: strings.Repeat("a", 256), wantErr: true},
}
```

**Porquê:** Testes pesados demais tornam a suite lenta e frágil. Testes redundantes adicionam custo de manutenção sem adicionar proteção.

---

## Quick-reference: regra -> expectativa

| Aspecto | Expectativa |
|---------|-------------|
| Formato de teste | Table-driven com `t.Run()` para multi-cenário |
| Estrutura | Table-driven (principal); AAA explícito como fallback para testes não table-driven |
| Nomes top-level | `Test<Type>_<Method>` ou `Test<Function>` |
| Nomes de subteste | Frases descritivas em inglês, sem PascalCase |
| Variável de iteração | Consistente dentro do arquivo (`tc` ou `tt`) |
| Framework de mock | `uber-go/mock` com `//go:generate mockgen` |
| Context | `t.Context()` (nunca `context.Background()`) |
| Paralelismo | `t.Parallel()` em top-level e subtestes |
| Assertions | `testify/assert` + `testify/require` para pré-condições |
| Helpers | Devem chamar `t.Helper()` |
| Controller | `gomock.NewController(t)` sem `defer ctrl.Finish()` |
| Cenários mínimos | Happy path + error cases + validação + edge cases |
| Mock specificity | Argumentos específicos; evitar `gomock.Any()` salvo context |
| Determinism | Clock e randomness injetados; sem `time.Sleep` em testes |
| Nível de teste | Unitário para lógica pura; integração só quando necessário |
