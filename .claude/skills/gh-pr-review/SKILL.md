---
name: gh-pr-review
description: "Analisa comentários de revisão de PRs no GitHub, planeja correções e verifica status de CI."
---

Fluxo para analisar e endereçar feedback de revisão em Pull Requests usando `gh` CLI.

## Pré-requisitos

Antes de iniciar, verificar se as ferramentas necessárias estão configuradas. Se alguma não estiver disponível, informar o usuário **explicitamente** com a mensagem correspondente.

### `gh` CLI

Necessário para todo o fluxo (buscar PR, comentários, checks). Verificar com `gh auth status`.

Se não estiver instalado ou autenticado, informar o usuário:

> `gh` CLI não está configurado. Instale via `brew install gh` e autentique com `gh auth login`.

### `SONAR_TOKEN` (opcional)

Necessário para buscar findings detalhados da API do SonarCloud. Se a variável de ambiente não estiver definida, informar o usuário:

> Variável `SONAR_TOKEN` não está definida. Para buscar findings diretamente da API do SonarCloud, configure-a com um token válido: `export SONAR_TOKEN=<seu-token>`. Sem ela, apenas o fallback via comentário do bot na PR estará disponível.

---

## Workflow

```
Verificar pré-requisitos -> Fetch PR data -> Analisar comentários -> Planejar correções -> Responder comentários -> Executar
```

---

## 1. Buscar dados da PR

### Detalhes e comentários gerais

```bash
gh pr view <N> --comments --json title,body,comments,reviews,url
```

### Comentários inline de revisão (code review)

```bash
gh api --paginate repos/{owner}/{repo}/pulls/<N>/comments \
  --jq '.[] | "---\nID: \(.id) | **\(.user.login)** on `\(.path):\(.line // .original_line)`:\n\(.body)\n"'
```

> Nota: o campo `id` é necessário para responder a comentários via API (ver seção "Responder comentários").

Para detectar `{owner}/{repo}` automaticamente:

```bash
gh repo view --json nameWithOwner -q '.nameWithOwner'
```

### Status de checks/CI

```bash
gh pr checks <N>
```

### Findings do SonarCloud

Extrair automaticamente do repositório:

```bash
grep '^sonar.projectKey=' .sonarcloud.properties | cut -d= -f2
```

#### Via API (requer SONAR_TOKEN)

```bash
curl -s -u "${SONAR_TOKEN}:" \
  "https://sonarcloud.io/api/issues/search?componentKeys=<projectKey>&pullRequest=<N>&resolved=false"
```

Campos úteis na resposta:

| Campo | Descrição |
|-------|-----------|
| `component` | Arquivo onde o finding foi detectado |
| `line` | Linha do código |
| `message` | Descrição do problema |
| `severity` | BLOCKER, CRITICAL, MAJOR, MINOR, INFO |
| `type` | BUG, VULNERABILITY, CODE\_SMELL |
| `effort` | Estimativa de tempo para corrigir |

#### Fallback: comentário do bot na PR (sem SONAR_TOKEN)

Se `SONAR_TOKEN` não estiver configurado, extrair informações do comentário do bot SonarCloud na PR:

```bash
gh pr view <N> --comments --json comments \
  --jq '.comments[] | select(.author.login == "sonarcloud[bot]") | .body'
```

> Nota: o fallback fornece apenas um resumo (quality gate, coverage). Para findings detalhados (arquivo, linha, severidade), a API com `SONAR_TOKEN` é necessária.

---

## 2. Analisar comentários

Categorizar cada comentário:

| Categoria | Exemplos | Ação |
|-----------|----------|------|
| Bot/CI informacional | moonlight-pipeline, sonarcloud | Apenas reportar status (coverage, quality gate) |
| Reviewer automatizado | Copilot | Validar contra o código real; triagem de falsos positivos |
| Reviewer humano | Membros do time | Sempre considerar acionável; separar sugestões de mudanças obrigatórias |

### Triagem de falsos positivos

Para comentários de reviewers automatizados (ex: Copilot):

1. Ler o código referenciado no comentário (`path:line`)
2. Verificar se a observação é factualmente correta
3. Cruzar com a arquitetura real (ex: esquema de GSI no DynamoDB, interfaces existentes)
4. Apresentar ao usuário quais comentários são acionáveis vs falsos positivos, com justificativa
5. Perguntar ao usuário antes de descartar qualquer comentário

---

## 3. Planejar correções

1. Apresentar resumo categorizado dos comentários ao usuário
2. Para cada comentário acionável, descrever a mudança necessária
3. Perguntar ao usuário quais correções executar
4. Criar todo list com as tarefas aprovadas

---

## 4. Responder comentários

### Responder a comentários inline de revisão

Para responder a um comentário inline, usar o campo `in_reply_to` com o `id` do comentário original:

```bash
gh api repos/{owner}/{repo}/pulls/<N>/comments \
  -f body="<mensagem>" \
  -F in_reply_to=<comment_id>
```

Para responder a múltiplos comentários em lote:

```bash
for id in <id1> <id2> <id3>; do
  gh api repos/{owner}/{repo}/pulls/<N>/comments \
    -f body="<mensagem>" \
    -F in_reply_to=$id
done
```

### Antes de responder

- **SEMPRE** perguntar ao usuário antes de enviar respostas.
- Apresentar a mensagem proposta e a lista de comentários que serão respondidos.
- Confirmar se o usuário deseja responder a todos, a um subconjunto, ou ajustar a mensagem.

### Resolver conversas

A API **REST** do GitHub não suporta resolver conversas (marcar como "Resolved"). Porém, é possível via **GraphQL** usando a mutation `resolveReviewThread`.

#### Buscar thread IDs

```bash
gh api graphql -f query='
query {
  repository(owner: "{owner}", name: "{repo}") {
    pullRequest(number: <N>) {
      reviewThreads(first: 100) {
        nodes {
          id
          isResolved
          comments(first: 1) {
            nodes { body author { login } }
          }
        }
      }
    }
  }
}'
```

#### Resolver uma thread

```bash
gh api graphql -f query='
mutation {
  resolveReviewThread(input: { threadId: "<THREAD_NODE_ID>" }) {
    thread { isResolved }
  }
}'
```

---

## 5. Executar correções

Seguir as convenções do projeto definidas em CLAUDE.md, com atenção aos comandos definidos e formatação.
