---
name: arch-review
description: "Revisa arquitetura de módulos Go: direção de dependências, segregação de interfaces e boundaries de domínio."
---

Revisão arquitetural focada nos princípios gerais do projeto, especialmente baseados em DDD tático e Portas e Adaptadores.
Complementa linters e testes estáticos ao verificar aspectos que exigem julgamento humano:
boundaries de domínio, direção de dependências, riqueza de entidades e segregação de interfaces.

> Inspirado em [cab-killer](https://github.com/bdfinst/cab-killer) conforme [artigo](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how) 
> do autor: automatizar o que pode ser automatizado, reservar julgamento humano para o que genuinamente o requer.

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
          "file": "path/to/file.go",
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
- `error`: Violação que causa acoplamento entre contextos, vazamento de abstrações, ou código não testável.
- `warning`: Lógica mal posicionada ou abstração ausente que adiciona fricção ao desenvolvimento.
- `suggestion`: Melhoria de modelagem sem impacto imediato.

**Confidence:**
- `high`: Fix mecânico e objetivo (mover import, adicionar interface guard, relocar struct, adicionar mapeamento).
- `medium`: Direção do fix é clara mas o escopo de refatoração pode variar.
- `none`: Requer julgamento humano (trade-off de boundary, exceção pragmática, decisão de modelagem).

**Status geral:**
- `pass`: Nenhum issue encontrado.
- `warn`: Apenas warnings e suggestions.
- `fail`: Pelo menos um error encontrado.
- `skip`: Nenhum arquivo `.go` encontrado no escopo alvo, ou alvo não é um módulo revisável.

Retornar `{"status": "skip", "categories": [], "summary": "No Go files in target"}` quando nenhum arquivo Go for encontrado no path alvo.

Categorias sem issues devem ser omitidas do output. Se o status geral for `pass`, listar apenas o resultado e um resumo positivo breve.

---

## Workflow de exploração

Antes de detectar issues, mapear a estrutura do módulo alvo:

### 1. Identificar camadas

Usar Glob e Grep para localizar cada camada:

**Domínio (entidades, value objects, interfaces):**
```
<modulo>/*.go (excluindo *_test.go, service.go)
```
Procurar por: definições de `type ... struct`, `type ... interface`, constantes de domínio, métodos com receiver.

**Serviços / Use Cases:**
```
<modulo>/service.go ou <modulo>/*_service.go
```
Procurar por: interface guards (`var _ ... = (*...)(nil)`), construtores `New*`, métodos que orquestram.

**Infraestrutura (repositórios, clientes externos):**
```
<modulo>/dynamodb/*, <modulo>/postgres/*, <modulo>/*/repository.go
```
Procurar por: implementações de interfaces do domínio, modelos de persistência internos.

**Integrações:**
```
<modulo>/*/service.go (subpacotes como gmud/, notification/, etc.)
```
Procurar por: interfaces locais, clientes para serviços externos.

### 2. Mapear dependências

Para cada arquivo `.go` no módulo, verificar imports:
- Anotar quais pacotes do projeto são importados.
- Construir um grafo de dependências simplificado.
- Identificar a direção: domínio deve ser importado, nunca importar infra.

### 3. Catalogar interfaces e implementações

Listar:
- Interfaces definidas e em qual pacote vivem.
- Structs que as implementam (verificar interface guards).
- Input/output structs e onde estão definidas.

### 4. Verificar se é um módulo de domínio

Se o módulo não contém entidades com comportamento, interfaces de use case, ou repositórios,
ele provavelmente não é um módulo de domínio. Neste caso, as categorias 1-9 não se aplicam.
Aplicar apenas a categoria 10 (anti-patterns de AI) e as regras Go gerais de CLAUDE.md.

Ver nota de escopo em `references/go-domain-module-conventions.md`.

---

## Categorias de revisão

### 1. Violações de boundary de domínio

**Severidade**: error/warning

**Detectar:**
- Lógica de negócio em handlers HTTP (route handlers computando validações de domínio, autorização, transformações).
- Lógica de negócio em camada de repositório ou acesso a dados.
- Application services contendo regras de negócio -- services devem orquestrar objetos do domínio e infraestrutura, não possuir regras. Exceção: domain services que legitimamente possuem regras que não pertencem a uma única entidade.

Ver exemplos corretos e anti-patterns em `references/go-domain-module-conventions.md`: **Pattern 1** (Rich Domain Model).

### 2. Direção de dependências

**Severidade**: error

**Detectar:**
- Pacote de domínio importando pacotes de infraestrutura (ex: `maintenance` importando `maintenance/dynamodb`).
- Dependências circulares entre pacotes.
- Pacote de domínio importando frameworks HTTP, SDKs de cloud, ou ORMs diretamente.

**Regra:** A direção das setas deve ser sempre para dentro (em direção ao domínio).

Ver grafo completo em `references/go-domain-module-conventions.md`: seção **Grafo de dependências esperado**.

### 3. Posicionamento de interfaces

**Severidade**: warning

**Detectar:**
- Interfaces de use case definidas fora do pacote de domínio.
- Interface de repositório definida no pacote de infraestrutura em vez do domínio.
- Interface definida longe de quem a consome (princípio: defina interfaces no consumidor).

**Regra:**
- `EventRepository`, use case interfaces (`*UseCase`) -> vivem no pacote de domínio.
- `DynamoClient`, `JiraClient` (wrappers de SDK) -> vivem no pacote de infraestrutura que os usa.
- `Creator`, `Publisher` (integrações) -> vivem no pacote que define o contrato.

### 4. Segregação de interfaces

> *"The bigger the interface, the weaker the abstraction."* — Rob Pike

**Severidade**: warning

**Detectar:**
- Uma única interface com muitos métodos usada por consumidores que precisam de poucos.
- Consumers dependendo do tipo concreto em vez da interface mais estreita.
- Handlers/subscribers importando o service concreto em vez da use case interface.

Ver exemplos corretos e anti-patterns em `references/go-domain-module-conventions.md`: **Pattern 2** (Interfaces de use case segregadas).

### 5. Comportamento de entidades (Rich vs Anemic)

**Severidade**: warning

**Detectar:**
- Entidades que são puro data holders (apenas campos, sem métodos de comportamento).
- Toda lógica vivendo em services enquanto entidades são structs passivas.
- Mutação direta de estado interno sem métodos com intenção revelada.
- Ausência de state machines para entidades com lifecycle (status transitions).

**Regra:** Invariantes de negócio e transições de estado devem ser métodos na entidade.

Ver exemplos corretos e anti-patterns em `references/go-domain-module-conventions.md`: **Pattern 1** (Rich Domain Model).

### 6. Colocalização de input/output structs

**Severidade**: suggestion

**Detectar:**
- Input structs para use cases definidas em pacotes separados (ex: pacote `dto/` ou `request/`).
- Input structs distantes da interface que as consome.
- Domain objects usados diretamente como DTOs de transferência entre camadas.

**Regra:** Input/output structs vivem no mesmo arquivo ou pacote da interface que as usa.

Ver exemplos corretos e anti-patterns em `references/go-domain-module-conventions.md`: **Pattern 5** (Input structs co-localizadas).

### 7. Domain events

**Severidade**: warning

**Detectar:**
- Operações de escrita que alteram estado sem publicar domain events.
- Domain events publicados antes da persistência (risco de inconsistência).
- Ausência de payloads tipados para eventos (uso de `map[string]any` ou similar).
- Comunicação direta entre bounded contexts onde domain events seriam apropriados.

**Regra:** Domain events devem ser publicados após persistência bem-sucedida.
Falha na publicação deve propagar como erro.

### 8. Encapsulamento de infraestrutura

> *"A little copying is better than a little dependency."* — Rob Pike

**Severidade**: warning

**Detectar:**
- SDKs externos usados diretamente sem interface wrapper local.
- Ausência de interface guards (`var _ Interface = (*Impl)(nil)`).
- Modelos de persistência (com tags `dynamodbav`, `json`, `gorm`) expostos fora do pacote de infra.
- Funções de mapeamento (domain <-> persistence model) ausentes.

**Regra:**
- SDKs externos devem ser acessados via interface wrapper local.
- Modelos de persistência devem ser internos ao pacote de infra.
- Mapeamento bidirecional (domain <-> persistence) deve existir.
- Interface guards devem estar presentes.

Ver exemplos em `references/go-domain-module-conventions.md`: **Pattern 4** (Interface guards), **Pattern 7** (Interface local para SDK), **Pattern 8** (Modelo de persistência interno).

### 9. Naming e linguagem ubíqua

**Severidade**: suggestion

**Detectar:**
- Terminologia inconsistente para o mesmo conceito dentro do módulo (ex: `Order` em um lugar, `Purchase` em outro).
- Nomes genéricos que obscurecem intenção: `process`, `handle`, `data`, `info`, `manager`, `helper`, `utils`.
- Nomes de pacotes que não comunicam o bounded context.

**Não flagear:** Terminologia como "errada" baseado em suposições de negócio. Apenas flagear inconsistência interna observável no código.

### 10. Anti-patterns de AI

> *"Clear is better than clever."* — Rob Pike

**Severidade**: warning

**Detectar:**
- Abstrações desnecessárias: factories de uso único, wrappers que apenas delegam sem adicionar valor.
- Indireção redundante: interface com uma única implementação que nunca será substituída e não facilita testes.
- Over-engineering de patterns: Strategy pattern para 2 casos, Builder para struct com 3 campos.
- Código que "parece" arquiteturalmente correto mas adiciona complexidade sem benefício.

**Não flagear:** Interfaces com uma implementação que existem para testabilidade (mock injection). O teste é o valor.

---

## Ignore

Não flagear os seguintes itens (tratados por outros agentes ou fora do escopo):

- **Código gerado**: Arquivos em `mocks/`, código gerado por `go:generate`.
- **Módulos não-domínio**: `cmd/main.go`, config loaders triviais, wire files, pacotes utilitários — aplicar apenas categoria 10 e regras Go gerais.
- **Internals de bibliotecas de terceiros**: Não avaliar se a lib está sendo usada corretamente; focar na arquitetura do projeto.
- **Code style puro**: Formatação, import ordering, line length — tratados por `gofmt`, `goimports`, e linters.
- **Terminologia de negócio**: Não flagear termos como "errados" baseado em suposições; apenas inconsistência interna observável (ver categoria 9).

---

## Instruções

1. **Sempre explorar antes de julgar.** Mapear a estrutura completa antes de emitir qualquer finding.
2. **Ser específico.** Cada finding deve referenciar arquivo e linha exatos.
3. **Justificar cada finding.** Explicar por que é um problema, não apenas que é.
4. **Sugerir fix concreto.** Não apenas "mova isso", mas mostrar onde e como.
5. **Respeitar pragmatismo.** Nem toda "violação" precisa ser corrigida. Se há razão pragmática (ex: constantes de config em service), notar mas não flagear como error.
6. **Não inventar problemas.** Se o módulo está bem estruturado, retornar `pass`.
7. **Consultar CLAUDE.md** para convenções Go específicas do projeto.
8. **Referenciar patterns concretos** do arquivo `references/go-domain-module-conventions.md` quando aplicável.
