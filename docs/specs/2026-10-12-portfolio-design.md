# Portfolio: Design (spec)

**Data:** 2026-10-12
**Status:** Aprovado para planejamento

Vocabulário em [GLOSSARY.md](../../GLOSSARY.md). Decisões estruturais em [docs/adr](../adr/).

## Objetivo

Site pessoal em `wendellopes.dev` para conseguir trabalho: uma Home de posicionamento voltada a recrutadores e um Blog próprio no mesmo domínio. Inspirado em infoslack.pro, guardadas as proporções. Restrição principal: custo mínimo na AWS, sem contar com free tier.

## Decisões

| Tema | Decisão |
| --- | --- |
| Tipo de Site | Híbrido: Home de posicionamento + `/blog` |
| Público | Recrutadores e engenheiros avaliando o Wendel |
| Idiomas | PT-BR (padrão) e EN, URLs `/pt/...` e `/en/...`, botão troca. `/` vai para PT. Sem detecção pelo navegador |
| Home | Hero, faixa de Empresas (PicPay, Nubank), 5 Camadas, Timeline, seção "construindo agora", contato |
| Hero (PT) | "Construo sistemas de pagamento que milhões de pessoas usam sem perceber." / "Engenheiro de software sênior e Tech Lead. Nove anos em fintech, com passagens por PicPay e Nubank." |
| Texto | Proibido travessão (—) em qualquer texto do Site |
| Visual | Base estética do infoslack; azul `#2563EB`; claro/escuro seguindo o sistema com botão; Space Grotesk, Inter, JetBrains Mono |
| Contato | E-mail, LinkedIn, GitHub, dev.to. Sem telefone. Foto no hero. CV em PDF por Idioma |
| Frontend | React + Vite com SSG (HTML pré-renderizado) |
| Backend | Go, uma Lambda (router `net/http`), Function URL atrás do CloudFront com OAC |
| Dados | DynamoDB on-demand, single-table + 1 GSI |
| Posts | Escritos no Painel, guardados no DynamoDB, corpo em Markdown ([ADR 0001](../adr/0001-posts-no-dynamodb-com-html-estatico.md)) |
| Painel | Só local, sem auth própria; usa o perfil AWS do Wendel ([ADR 0003](../adr/0003-painel-so-local.md)) |
| Rebuild | Painel local roda `gh workflow run deploy.yml`; o workflow exporta, gera o HTML, envia ao S3 e invalida o CloudFront |
| Imagens de Post | Arquivos no repo em `web/public/blog/<post>/` (sem upload no v1) |
| Infra | CloudFormation puro, `aws cloudformation deploy` via Makefile, tudo em us-east-1 |
| DNS | Domínio e zona no Route 53 |
| Local | MiniStack emula as stacks `data` e `app` |
| CI/CD | GitHub Actions, repo público, OIDC para AWS, deploy só a partir da `main` |
| Observabilidade | `slog` JSON no CloudWatch, retenção de 14 dias. Sem X-Ray/OTel no v1 |
| Testes | Go: convenções do pricemonitor + integração contra MiniStack. Web: Vitest + Testing Library. E2E: Playwright |
| Custo alvo | Cerca de US$ 0,60/mês + domínio (US$ 17/ano). Budget com alerta em US$ 2 |

## Domínio

- **Post** tem `id`, Visualizações e 1 ou 2 **Versões** (PT, EN).
- **Versão** tem título, slug, corpo Markdown, estado (**Rascunho** | **Publicada**) e data de publicação.
- O slug é imutável depois da primeira publicação da Versão.
- Despublicar devolve a Versão a Rascunho. Excluir um Post é definitivo e permitido em qualquer estado; se havia Versão Publicada, dispara rebuild.
- A contagem de Visualizações é do Post, somando as Versões. É aproximada: sem cookie, sem deduplicação, com filtro simples de bots por User-Agent.

## Modelo DynamoDB

Tabela única, on-demand.

| Item | PK | SK | GSI1PK | GSI1SK |
| --- | --- | --- | --- | --- |
| Post | `POST#<id>` | `META` | | |
| Versão | `POST#<id>` | `VERSION#<lang>` | `LANG#<lang>#PUBLISHED` (só se Publicada) | `<publishedAt>` |

Padrões de acesso:

1. Painel lista Posts com o estado das Versões: `Scan` (escala de dezenas de itens, só local).
2. Painel abre um Post: `Query PK=POST#<id>`.
3. Export lista as Versões Publicadas de um Idioma, das mais recentes para as mais antigas: `Query GSI1 PK=LANG#<lang>#PUBLISHED`, `ScanIndexForward=false`.
4. Contador: `UpdateItem ADD views :1` no item `META` (fora do GSI, sem escrita duplicada).

## API

Uma só aplicação `http.Handler`. Rotas admin existem no código e rodam só localmente (não registradas no handler da Lambda).

| Rota | Onde | Faz |
| --- | --- | --- |
| `POST /api/posts/{id}/views` | Lambda + local | soma 1 e devolve `{"views": n}` |
| `GET /api/admin/posts` | local | lista Posts |
| `POST /api/admin/posts` | local | cria um Post com uma Versão em Rascunho |
| `GET /api/admin/posts/{id}` | local | abre o Post |
| `PUT /api/admin/posts/{id}/versions/{lang}` | local | cria ou edita a Versão |
| `POST /api/admin/posts/{id}/versions/{lang}/publish` | local | publica e dispara rebuild |
| `POST /api/admin/posts/{id}/versions/{lang}/unpublish` | local | despublica e dispara rebuild |
| `DELETE /api/admin/posts/{id}` | local | exclui; dispara rebuild se havia Versão Publicada |

O front chama o contador com `fetch(..., {method: "POST", keepalive: true})` e envia o header `x-amz-content-sha256` (exigido pelo OAC do CloudFront para Lambda em POST).

## Repositório

```
post/              domínio: Post, Versão, Service, interfaces (ports)
post/dynamodb/     implementação do Repository
internal/dynamodb  building block técnico (cliente, marshalling)
internal/http      router e handlers (driving adapter)
cmd/api/           main: Lambda em prod, servidor HTTP local
cmd/export/        exporta Versões Publicadas para JSON consumido pelo SSG
web/               React + Vite + SSG (Site público + Painel em /painel, só dev)
web/e2e/           Playwright
infra/             templates CloudFormation
docs/              specs, ADRs, setup
.claude/, CLAUDE.md   adaptados do pricemonitor
```

Adaptações do `CLAUDE.md` do pricemonitor: trocar `pperr`/`instrument` por `fmt.Errorf("%w")` e `log/slog`; acrescentar seções de React/TypeScript, CloudFormation e a regra do travessão. Mantém a regra: a IA não faz commit, push, merge nem rebase.

## Stacks CloudFormation

| Stack | Conteúdo | MiniStack |
| --- | --- | --- |
| `portfolio-foundation` | certificado ACM (validação DNS na zona existente), bucket de artefatos, provedor OIDC do GitHub, role de deploy, AWS Budget | não |
| `portfolio-data` | tabela DynamoDB, bucket do Site (`DeletionPolicy: Retain`) | sim |
| `portfolio-app` | Lambda Go (arm64, `provided.al2023`), Function URL, role, log group (14 dias), reserved concurrency 5 | sim |
| `portfolio-edge` | CloudFront, OAC para S3 e Lambda, registros DNS (A/AAAA alias) | não |

A zona do Route 53 é criada pelo registro do domínio e entra nas stacks como parâmetro `HostedZoneId`.

## Segurança do repositório

- Ruleset na `main`: PR obrigatório, CI verde obrigatório, sem force push, sem deleção, bypass só para o dono.
- Actions: aprovação obrigatória para workflows de colaboradores externos; nunca `pull_request_target`; permissões padrão de `GITHUB_TOKEN` só leitura.
- OIDC: trust policy da role de deploy aceita só `repo:<owner>/portfolio:environment:production`; o environment `production` só aceita a branch `main`.
- Nenhum segredo no repo.
- Workflows do Claude, se mantidos, só reagem a `author_association == 'OWNER'`.

## Entrega

0. **Spike MiniStack:** `data` + `app` sobem via CloudFormation no MiniStack; repository Go grava e lê.
1. **Blueprint:** esqueleto do repo, `CLAUDE.md`, Makefile, CI, rulesets.
2. **Home bilíngue estática em produção:** `foundation` + `data` (bucket) + `edge`. Site no ar.
3. **API de Posts + Painel local.**
4. **Export + páginas do Blog + RSS por Idioma + rebuild.**
5. **Visualizações:** stack `app` em produção.
6. **Playwright, importação RSS no dev.to, polimento.**

## Fora do escopo do v1

Painel na nuvem e login, upload de imagens, tags, busca, comentários, newsletter, detecção de Idioma, analytics além do contador, X-Ray/OTel, ambiente de staging.

## Pendências do Wendel

- Foto para o hero.
- Logos de PicPay e Nubank (preferencialmente SVG).
- CV em PT-BR (PDF).
