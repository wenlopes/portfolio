# Portfolio: instruções para agentes de IA

Site pessoal do Wendel (Home bilíngue + Blog). Spec em `docs/specs/`, vocabulário em `GLOSSARY.md`, decisões em `docs/adr/`. Regras de código herdadas do projeto pricemonitor e adaptadas.

- Você é especialista em Go, React/TypeScript e AWS (CloudFormation, Lambda, DynamoDB).
- Mantenha as respostas concisas e focadas em soluções técnicas.
- **A IA NÃO está autorizada a executar `git commit` (nem `git push`, `git merge`, `git rebase` ou qualquer operação que altere o histórico/estado do repositório).** Toda e qualquer operação de versionamento será feita pelo usuário. A IA pode gerar e modificar arquivos (incluindo arquivos gerados como mocks e docs) e deixá-los prontos no working tree, mas o ato de commitar é exclusivo do usuário.

## Agent skills

### Issue tracker

Issues e specs ficam no GitHub Issues de wenlopes/portfolio, via `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

Labels padrão: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` na raiz. See `docs/agents/domain.md`.

### Revisões

- `/review-arch [path]`: arquitetura e boundaries (skill `arch-review`).
- `/review-tests [path]`: qualidade de testes (skill `test-review`).
- Skill `gh-pr-review`: revisão de PR no GitHub. Skill `explain-code`: explicar código com analogia e diagrama.

## Texto do Site

- **Proibido travessão (—) em qualquer texto do Site** (Home, Posts, metadados, alt text, RSS). Use vírgula, dois-pontos, ponto ou parênteses.
- Use o vocabulário do `GLOSSARY.md` em textos de domínio (Post, Versão, Rascunho, Publicada, Visualização, Idioma).

## Arquitetura de Código

### Organização de Pacotes

- **Pacotes de domínio ficam na raiz do projeto.** Cada contexto de domínio é um pacote na raiz (ex: `post/`), contendo: os tipos/entidades, as interfaces (ports), as implementações dessas interfaces (adapters, expressos em termos de domínio) e os services com a lógica de negócio. Isso deixa o domínio explícito e declarativo no topo da árvore.
- **`internal/` é para building blocks técnicos**, acoplados a uma lib ou sistema externo e **agnósticos ao domínio**: mapeamento/integração de persistência (ex: DynamoDB), clients de integração externa (ex: GitHub, HTTP), helpers de parsing, observabilidade, etc. Não conhecem tipos de domínio: operam em bytes, rows e payloads.
- **A lógica de negócio vive nos services do domínio.** O service recebe as interfaces (nunca a implementação concreta nem um pacote `internal/` diretamente) e opera sobre os tipos de domínio.
- **As implementações das interfaces ficam no pacote de domínio, mas delegam o trabalho técnico para `internal/`.** Exemplo: o domínio define a interface `Repository`; a struct que a implementa vive no pacote de domínio e traduz entidades de domínio, mas por baixo dos panos usa `internal/dynamodb` para falar com o banco. Assim o detalhe da lib fica isolado em `internal/`, e o domínio expõe contrato + tradução para suas entidades.
- **Cada implementação concreta de um port fica em um sub-pacote do domínio nomeado pelo backend/canal** (ex: `post/dynamodb/`). Trocar de backend = adicionar um sub-pacote irmão (ex: `post/postgres/`). As regras do pacote (interfaces, tipos de domínio) e o `Service` ficam nos arquivos raiz do domínio (`<dominio>.go`, `service.go`). Crie o sub-pacote apenas quando há de fato uma implementação concreta a isolar; não force um sub-pacote (ou um par em `internal/`) quando a lib já é alto nível e o wrapper seria vazio.
- **Direção de dependência:** o domínio **importa** `internal/` (consome os building blocks); `internal/` **nunca importa** o domínio. Exceção: *driving adapters* (ex: server HTTP em `internal/http`) que acionam o domínio podem importá-lo. Mantenha building blocks genéricos (ex: um scheduler que recebe `func(context.Context) error`) sem dependência do domínio.

### Estrutura de Arquivos

Abaixo está um exemplo de estrutura de arquivos recomendada para pacotes Go, alguns blocos são opcionais dependendo da complexidade do pacote, quantidade de interfaces e implementações, etc., mas considerar seguir este padrão para manter a consistência:

```go
// Interface guard.
var _ Interface = (*Implementation)(nil)

// Constructor.
func NewImplementation(...) *Implementation { ... }

// Implementation struct.
type Implementation struct { ... }

// Interface definition
type SomeInterface interface { ... }

// Input structs
type SomeInput struct { ... }
```

### Interfaces e Contratos

- Implemente validação de interface com `var _ Interface = (*Implementation)(nil)` (interface guard).
- Defina interfaces após as implementações quando no mesmo arquivo.
- Métodos sempre recebem `context.Context` como primeiro parâmetro.
- Prefira input structs em vez de múltiplos parâmetros; para construtores com muitos parâmetros opcionais, considere o Functional Options Pattern.
- Retorne apenas `error` para operações de validação/execução.
- Aceite interfaces e retorne tipos concretos.
- Crie structs dedicadas para inputs de métodos (ex: `PutVersionInput`).
- Mantenha inputs próximos à interface que os utiliza.

Interface Guards garantem que uma struct implementa uma interface em tempo de compilação:

```go
// Ensures Repository implements post.Repository.
var _ post.Repository = (*Repository)(nil)

// Multiple interface implementations.
var (
    _ http.Handler = (*Handler)(nil)
    _ Rebuilder    = (*GitHubRebuilder)(nil)
)
```

### Injeção de Dependências

- Construa dependências complexas dentro do construtor usando `New*` functions.
- Ao usar uma configuração global, injete uma struct de ambientes (`Environments`), e não valores individuais.

### Functional Options Pattern

Use o padrão Functional Options para construtores que necessitam de configuração flexível. Este padrão permite evolução da API sem quebrar retrocompatibilidade.

**Quando usar:**

- Construtores com muitos parâmetros opcionais.
- APIs que precisam manter retrocompatibilidade ao adicionar novos parâmetros.
- Configurações que se beneficiam de legibilidade no código consumidor.

**Estrutura padrão:**

```go
// ServerOption configures Server parameters.
type ServerOption func(*Server)

// WithTimeout sets the server timeout.
func WithTimeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

// NewServer creates a new Server with the given options.
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		timeout: 30 * time.Second, // default value
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```

**Uso:**

```go
server := NewServer(
	WithTimeout(time.Minute),
	WithMaxConn(100),
)
```

**Convenções:**

- Prefixe funções de opção com `With`.
- Documente cada função de opção com comentário godoc.
- Defina valores padrão sensatos no construtor.
- Use `<Type>Option` para o tipo da função de opção.

## Padrões de Código

### Convenções Gerais

- Comentários de código devem ser **SEMPRE** escritos em inglês.
- Comentários devem terminar com ponto final.
- **SEMPRE** use `any` em vez de `interface{}`.
- Structs que implementam interfaces devem definir um Interface Guard.
- Use nomes de variáveis significativos e evite variáveis de uma letra, exceto para loops de curta duração.
  - **Bom**: `postRepository`, `publishedVersions`, `tableName`
  - **Ruim**: `pr`, `pv`, `t` (a menos que em contextos de curta duração)
  - **Aceitável**: `i` em `for i := 0; i < len(items); i++`
- Nomes de pacotes devem ser minúsculos, palavras únicas quando possível.
- Coloque os métodos públicos antes dos privados na definição da struct.
- Documente todos os métodos públicos com comentários godoc.
- Use [`gofmt`](https://golang.org/cmd/gofmt/) e [`goimports`](https://golang.org/x/tools/cmd/goimports) para formatar código consistentemente.
- Use `bytes.NewReader` (imutável) a menos que você realmente precise modificar o payload depois, via `bytes.NewBuffer`.

### Tratamento de Erros

- **SEMPRE** trate erros explicitamente; **NUNCA** os ignore.
- Use `errors.New` para erros sentinela e `fmt.Errorf` com o verbo `%w` para envolver erros ao adicionar contexto.
- Erros de domínio são sentinelas exportados no pacote de domínio (ex: `post.ErrNotFound`); adapters traduzem erros da lib para eles.
- Use `errors.Is()` e `errors.As()` para verificação de erros.

### Logging

- Use `log/slog` com handler JSON (vai para o CloudWatch, retenção de 14 dias).
- Escreva mensagens de log limpas e inclua contexto relevante como atributo:
```go
		slog.ErrorContext(ctx, "something bad happened",
			slog.Any("error", err),
			slog.Int("http_status", resp.StatusCode),
			slog.String("post_id", postID),
		)
```
- Use `logger.With()` para criar um logger com atributos pré-configurados quando múltiplos logs compartilham o mesmo contexto.
- Log no nível apropriado: DEBUG, INFO, WARN, ERROR.

### HTTP

- Erros internos (500) retornam mensagem genérica; o erro original vai para o log.
- Valide todos os campos obrigatórios e retorne erros descritivos (400).
- Use códigos HTTP corretos (200, 201, 204, 400, 404, 409, 500).
- Rotas admin (`/api/admin/...`) só são registradas no servidor local, nunca no handler da Lambda (ADR 0003).

## Testes

### Padrões Gerais

- Use o comando `make test` para executar todos os testes (unit + integration + coverage). Testes de integração usam a build tag `integration` e rodam contra o MiniStack (`make up deploy-local` antes).
- **SEMPRE** use `t.Context()` em vez de `context.Background()` em testes para propagação adequada de contexto.
- Para nomes de subteste (`t.Run()`): seja legível, descritivo, conciso, evite caracteres especiais; espaços substituídos por underscores.
- Use testes orientados por tabela para testar múltiplos cenários.
- Teste tanto o caminho feliz quanto casos de erro.
- Use `t.Parallel()` para testes que podem executar em paralelo.
- **SEMPRE** atualize expectativas de teste ao refatorar código que muda formatos de saída, valores de retorno ou assinaturas de função.
- Teste as interfaces públicas, não implementações privadas. Teste métodos privados indiretamente através de interfaces públicas.
- Prefira o uso de `testify/assert` para aumentar a legibilidade dos testes.
- Evite usar `gomock.Any()` em expectativas de mock, a menos que absolutamente necessário. Seja específico sobre os argumentos esperados.

### Nomenclatura e Estrutura

| Aspecto | Melhor Prática |
| --- | --- |
| Nomes de teste de nível superior | Test + nome da função/método, PascalCase para casos específicos (ex: `TestListPublishedNewestFirst`). |
| Nomes de teste de método | Formato TestType_Method (ex: `TestRepository_PutVersion`). |
| Nomes de subteste | Legível, descritivo, conciso, evite caracteres especiais; espaços substituídos por underscores. |
| Estrutura de teste | Table-driven com `t.Run()`; AAA explícito como fallback para testes não table-driven. |
| Testes complexos | Extraia funções auxiliares para reduzir complexidade cognitiva ≤10. |

### Mocks

- Use `uber-go/mock` para gerar mocks.
- Adicione a diretiva `//go:generate mockgen ...` na interface que deseja mockar.
- Execute `make mocks` para gerar/atualizar todos os mocks.
- Mocks gerados devem ser versionados (o commit é feito pelo usuário).

## Frontend (React + TypeScript)

- Código em `web/`: React + Vite com SSG (HTML pré-renderizado). Painel em `/painel`, só em dev.
- TypeScript em modo `strict`. Proibido `any`; use `unknown` e estreite o tipo.
- Componentes funcionais e hooks. Um componente por arquivo, nome em PascalCase.
- Textos da Home ficam em arquivos por Idioma (`pt`, `en`), nunca hardcoded no componente.
- Testes com Vitest + Testing Library, pelo comportamento visível (papéis e textos acessíveis), não por detalhes de implementação.
- E2E com Playwright em `web/e2e/`.

## Infra (CloudFormation)

- CloudFormation puro em YAML, um template por stack em `infra/` (`foundation`, `data`, `app`, `edge`). Tudo em `us-east-1`.
- Deploy via Makefile (`aws cloudformation deploy`); nunca pelo console.
- Recursos com dados (tabela, bucket do Site) têm `DeletionPolicy: Retain` e `UpdateReplacePolicy: Retain`.
- Outputs compartilhados são exportados como `${AWS::StackName}-<Nome>` e consumidos com `Fn::ImportValue`.
- IAM com menor privilégio: ações e recursos explícitos, sem `*` quando houver ARN.
- Nenhum segredo no repo nem em template. Acesso do CI à AWS só via OIDC.
- `cfn-lint` deve passar (`make lint`). `data` e `app` devem subir no MiniStack (`make deploy-local`).

## Dependências e Fluxo de Trabalho

### Gerenciamento de Dependências

- Use Go modules para gerenciamento de dependências.
- Mantenha dependências mínimas e atualizadas.
- Prefira biblioteca padrão em vez de dependências externas quando possível.
- Use `make tidy` para limpar dependências.

### Executar pós mudanças

Execute na ordem: `gofmt -l .`, `make tidy`, `make test`, `make lint`, `make docs-build` (imprime o godoc de cada pacote e falha se algum não compila). O CI roda os mesmos passos. Todos devem passar sem erros. Deixe os arquivos gerados prontos no working tree para o usuário commitar.
