---
name: test-review
description: "Revisa qualidade de testes Go: table-driven, cobertura de cenários, mocks e proteção contra regressão."
---

Revisão de qualidade de testes focada em práticas que garantem segurança contra regressão.
Complementa cobertura de código (métricas numéricas) ao avaliar aspectos qualitativos:
completude de cenários, especificidade de mocks, resistência a refatoração e clareza de intenção.

> Testes de qualidade não apenas verificam que o código funciona hoje,
> mas garantem que mudanças futuras serão seguras. Um teste que quebra
> por refatoração interna (sem mudança de comportamento) é um teste frágil.

## Formato de saída

Produzir findings estruturados em JSON seguido de um resumo legível:

```json
{
  "status": "pass|warn|fail|skip",
  "categories": [
    {
      "name": "Category Name",
      "status": "pass|warn|fail",
      "issues": [
        {
          "severity": "error|warning|suggestion",
          "confidence": "high|medium|none",
          "file": "path/to/file_test.go",
          "line": 42,
          "message": "Descrição concisa do problema",
          "suggestedFix": "Como resolver"
        }
      ]
    }
  ],
  "summary": "2 warnings, 1 suggestion across 3 categories"
}
```

**Severidades:**
- `error`: Gap de cobertura crítico que permite regressões silenciosas, ou teste que mascara falhas reais.
- `warning`: Prática que reduz a eficácia dos testes ou adiciona fragilidade desnecessária.
- `suggestion`: Melhoria de legibilidade ou consistência sem impacto imediato na segurança.

**Confidence:**
- `high`: Fix mecânico e objetivo (adicionar assertion, substituir `context.Background()`, remover `defer ctrl.Finish()`).
- `medium`: Direção do fix é clara mas a estratégia de assertion ou reestruturação pode variar.
- `none`: Requer julgamento humano (escopo do teste, especificação de comportamento, trade-off de cobertura).

**Status geral:**
- `pass`: Nenhum issue encontrado.
- `warn`: Apenas warnings e suggestions.
- `fail`: Pelo menos um error encontrado.
- `skip`: Nenhum arquivo `_test.go` encontrado no escopo alvo.

Retornar `{"status": "skip", "categories": [], "summary": "No test files in target"}` quando nenhum arquivo de teste for encontrado no path alvo.

Categorias sem issues devem ser omitidas do output. Se o status geral for `pass`, listar apenas o resultado e um resumo positivo breve.

---

## Workflow de exploração

Antes de detectar issues, mapear a estrutura dos testes no escopo alvo:

### 1. Identificar arquivos de produção e testes correspondentes

Usar Glob para localizar pares:

**Arquivos de produção:**
```
<package>/*.go (excluindo *_test.go, mocks/)
```

**Arquivos de teste correspondentes:**
```
<package>/*_test.go
```

Verificar: todo arquivo `.go` com métodos públicos tem um `_test.go`? Listar arquivos sem teste correspondente.

### 2. Mapear cobertura de cenários

Para cada arquivo `_test.go`, verificar:
- Quais métodos/funções públicos do arquivo de produção são testados.
- Quantos cenários (subtestes) cada método possui.
- Se há cenários de happy path e error cases para cada método.
- Se edge cases e boundary conditions são cobertos.

### 3. Catalogar padrões de mock

Para cada arquivo `_test.go`, verificar:
- Qual framework de mock é usado (uber-go/mock, testify/mock, hand-written).
- Se mocks usam expectativas específicas ou `gomock.Any()` genérico.
- Se mocks são configurados por cenário (via `setupMock` func) ou compartilhados.
- Se `.AnyTimes()` é usado em operações de escrita.

### 4. Verificar estrutura e helpers

Verificar:
- Se testes usam table-driven pattern com `t.Run()`.
- Se funções auxiliares usam `t.Helper()`.
- Se `t.Parallel()` é usado consistentemente.
- Se `t.Context()` é usado em vez de `context.Background()`.

---

## Categorias de revisão

### 1. Cobertura de cenários

**Severidade**: error/warning

**Detectar:**
- Arquivo de produção com métodos/funções públicas sem nenhum `_test.go` correspondente (error).
- Funções de teste sem nenhuma chamada de assertion (`assert.*`, `require.*`) — um teste que apenas exercita código sem verificar resultados não oferece proteção contra regressão (error).
- Métodos públicos sem nenhum teste correspondente (error).
- Métodos testados apenas com happy path, sem cenários de erro (warning).
- Ausência de testes para validação de input (campos obrigatórios, valores inválidos) (warning).
- Ausência de testes para error propagation de dependências (repositório retorna erro, cliente externo falha) (warning).
- Ausência de testes para edge cases: lista vazia, nil, valores limite, string vazia (warning).
- Branches condicionais no código de produção (`if err`, `if x == nil`, `switch/case`, `default`) sem cenário de teste correspondente (warning).
- Transições de estado sem cobertura de caminhos inválidos (warning).

Ver tabela de cenários mínimos em `references/go-test-conventions.md`: **Pattern 7** (Cobertura de cenários).

### 2. Estrutura table-driven

**Severidade**: warning

**Detectar:**
- Múltiplos cenários para o mesmo método implementados como funções `Test*` separadas em vez de table-driven com `t.Run()`.
- Lógica de setup, execução e verificação entrelaçada dentro do loop body (ex: configurar mocks entre assertions, ou chamar o SUT entre blocos de setup).
- Subtestes que não são autocontidos (dependem de estado de subtestes anteriores).
- Uso de slice-based table tests (`[]struct`) quando map-based (`map[string]struct`) seria mais descritivo.
- Testes não table-driven com setup complexo que não seguem estrutura AAA explícita (Arrange, Act, Assert).

Ver exemplos em `references/go-test-conventions.md`: **Pattern 1** (Table-driven tests) e **Pattern 1b** (Fallback AAA).

### 3. Nomenclatura e legibilidade

**Severidade**: suggestion

**Detectar:**
- Nomes de teste top-level que não seguem `Test<Type>_<Method>` ou `Test<Function>`.
- Nomes de subteste vagos (`"test1"`, `"error case"`, `"success"`) que não descrevem o comportamento.
- Nomes de subteste em PascalCase (`"TestCreateWithInvalidInput"`) em vez de frases descritivas.
- Descrições misleading: nome do subteste descreve um comportamento, mas as assertions verificam outro.
- Inconsistência na variável de iteração: mistura de `tc` e `tt` no mesmo arquivo.
- Ausência de estrutura AAA explícita (`// Arrange`, `// Act`, `// Assert`) em testes não table-driven com setup complexo.

Ver convenções em `references/go-test-conventions.md`: **Pattern 2** (Nomenclatura de testes).

### 4. Especificidade de mocks

**Severidade**: warning/error

**Detectar:**
- Uso de `gomock.Any()` para argumentos que poderiam ser específicos (error quando o argumento é determinístico e conhecido).
- Uso de `.AnyTimes()` em operações de escrita (Create, Update, Delete, Publish) onde o número de chamadas é relevante (warning).
- Mocks configurados fora do loop de subtestes com `.AnyTimes()`, compartilhando expectativas entre cenários (warning).
- Ausência de verificação de argumentos passados ao mock quando o argumento é construído pela lógica sendo testada (warning).

**Não flagear:** `gomock.Any()` para `context.Context` como primeiro argumento.

**Substituição recomendada:** Valor determinístico conhecido → valor exato; struct construída pelo SUT → `gomock.AssignableToTypeOf(&Type{})`; matching parcial → custom `gomock.Matcher`; `context.Context` → manter `gomock.Any()`.

Ver exemplos em `references/go-test-conventions.md`: **Pattern 3** (Mocks específicos).

### 5. Testes frágeis (brittle tests)

**Severidade**: warning

**Detectar:**
- Uso de `gomock.InOrder` sem justificativa de negócio (testando ordem de implementação, não requisito).
- Assertions em representações string que mudam facilmente (ex: `assert.Equal(t, "expected full error message string here", err.Error())`).
- Testes que verificam campos internos de structs não exportados.
- Testes que replicam a lógica de implementação 1:1 (o teste é uma cópia do código).
- Assertions em valores exatos de timestamps ou UUIDs gerados.

**Não flagear:** `gomock.InOrder` quando a ordem é requisito de negócio (ex: persistir antes de publicar evento).

Ver discussão em `references/go-test-conventions.md`: **Pattern 8** (Resistência a regressão).

### 6. Independência e paralelismo

**Severidade**: warning

**Detectar:**
- Ausência de `t.Parallel()` no teste top-level.
- Ausência de `t.Parallel()` nos subtestes dentro de table-driven tests.
- Estado mutável compartilhado entre subtestes (variáveis modificadas no loop).
- Subtestes que dependem da ordem de execução de outros subtestes.
- Uso de variáveis de pacote mutáveis em testes sem cleanup.

Ver exemplos em `references/go-test-conventions.md`: **Pattern 5** (Paralelismo em testes).

### 7. Consistência de framework de mocks

**Severidade**: suggestion

**Detectar:**
- Mistura de `testify/mock` e `uber-go/mock` no mesmo pacote.
- Hand-written stubs/fakes coexistindo com mocks gerados para interfaces do mesmo pacote.
- Ausência de `//go:generate mockgen` em interfaces complexas que possuem mocks manuais.
- Uso de `uber-go/mock` para interfaces triviais (1-2 métodos) onde `testify/mock` inline seria mais simples.

**Não flagear:**
- Pacotes que usam `testify/mock` de forma consistente para cenários simples (interfaces com poucos métodos, mocks inline junto ao teste).
- Pacotes existentes que já adotaram um framework de forma consistente — a migração não é prioritária. Notar como suggestion apenas se houver mistura.

Ver convenção em `references/go-test-conventions.md`: **Pattern 9** (Consistência de framework).

### 8. Contexto e lifecycle

**Severidade**: warning

**Detectar:**
- Uso de `context.Background()` ou `context.TODO()` em testes onde `t.Context()` deveria ser usado.
- Funções auxiliares de teste que não chamam `t.Helper()`.
- Uso desnecessário de `defer ctrl.Finish()` quando `gomock.NewController(t)` já faz cleanup automático.
- Ausência de cleanup para recursos criados em testes (arquivos temporários, goroutines).

Ver exemplos em `references/go-test-conventions.md`: **Pattern 4** (Contexto e lifecycle).

### 9. Resistência a regressão

**Severidade**: warning/error

**Detectar:**
- Testes que apenas verificam `assert.NoError` sem validar o resultado retornado (error).
- Testes que verificam `assert.NotNil` sem verificar os campos relevantes do resultado (warning).
- Ausência de testes que validam o conteúdo de erros retornados (`errors.Is`, `errors.As`, ou verificação de mensagem) (warning).
- Testes de state machine que só cobrem transições válidas, sem testar transições inválidas (warning).
- Ausência de testes para cenários de concorrência onde optimistic locking é usado (warning).

**Regra:** Um teste que passa tanto para código correto quanto para código bugado é inútil.

### 10. Anti-patterns de AI em testes

> *"A test that never fails is a test that never helps."*

**Severidade**: warning

**Detectar:**
- Over-mocking: mocking de value objects, constantes, ou funções puras que não têm side effects.
- Test helpers excessivamente abstratos que obscurecem a intenção do teste (o leitor precisa navegar múltiplas funções para entender o cenário).
- Testes que espelham a implementação 1:1 (se a implementação chama A, B, C nessa ordem, o teste verifica exatamente A, B, C nessa ordem).
- Cenários de teste gerados mecanicamente sem valor real (testando getters/setters triviais).
- Mock expectations que repetem a lógica do código de produção como matchers.
- Blocos de assertions copy-paste que deveriam ser extraídos em um helper com `t.Helper()`.
- Valores literais mágicos em assertions sem explicação do seu significado (ex: `assert.Equal(t, 42, result)` — por que 42?).
- Test helpers ou utilities definidos mas nunca chamados (código morto de teste).

**Não flagear:** Testes de métodos que encapsulam regras de negócio ou invariantes (ex: validações, transições de estado) — estes devem ser testados mesmo que pareçam simples.

### 11. Non-determinism e flakiness

> *"A flaky test is worse than no test — it erodes trust in the entire suite."*

**Severidade**: error/warning

**Detectar:**
- Uso direto de `time.Now()` no código sob teste sem injeção de clock, tornando o teste dependente do momento de execução (warning).
- Uso de `rand.*` sem seed controlado ou injeção, produzindo resultados não-reproduzíveis (warning).
- `time.Sleep` no corpo do teste para sincronização (flakiness por timing) (error).
- Chamadas HTTP reais sem `httptest.Server` ou mock de transport (error).
- Conexões reais a banco de dados ou serviços externos sem test doubles (error).
- Goroutines lançadas no teste sem `sync.WaitGroup`, channel, ou mecanismo de sincronização (warning).
- Testes que dependem de ordem de execução ou estado externo compartilhado entre runs (warning).

Ver exemplos em `references/go-test-conventions.md`: **Pattern 11** (Non-determinism e flakiness).

### 12. Eficiência de nível de teste

**Severidade**: warning/suggestion

**Detectar:**
- Setup de integração (banco real, HTTP real, object graphs complexos) usado para testar lógica de uma única função — sugerir teste unitário com test double (warning).
- Testes que apenas exercitam comportamento de biblioteca de terceiros, não o código do projeto (suggestion).
- Múltiplos test cases que assertam o mesmo resultado com inputs diferentes sem que nenhuma boundary condition os diferencie — cobertura redundante (suggestion).

---

## Ignore

Não flagear os seguintes itens (tratados por outros agentes ou fora do escopo):

- **Código gerado**: Arquivos em `mocks/`, código gerado por `go:generate`.
- **Internals de bibliotecas de terceiros**: Não avaliar se a lib está sendo usada corretamente; focar nos testes do projeto.
- **Code style puro**: Formatação, import ordering, line length — tratados por `gofmt`, `goimports`, e linters.
- **Arquivos sem lógica testável**: `cmd/main.go`, config loaders triviais, wire files.

---

## Instruções

1. **Sempre explorar antes de julgar.** Mapear a estrutura completa dos testes antes de emitir qualquer finding.
2. **Ser específico.** Cada finding deve referenciar arquivo e linha exatos.
3. **Justificar cada finding.** Explicar por que é um problema de qualidade, não apenas que é.
4. **Sugerir fix concreto.** Não apenas "adicione mais testes", mas indicar quais cenários específicos faltam.
5. **Avaliar o par produção-teste.** Ler o arquivo de produção para entender quais branches e cenários o teste deveria cobrir.
6. **Respeitar pragmatismo.** Nem todo gap de cobertura precisa de teste. Se um método é trivial (ex: getter sem lógica), não flagear como error.
7. **Consultar CLAUDE.md** para convenções de teste específicas do projeto.
8. **Referenciar patterns concretos** do arquivo `references/go-test-conventions.md` quando aplicável.
