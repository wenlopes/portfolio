# Setup: do zero até o Site no ar

Passo a passo de tudo que só você pode fazer: conta AWS, domínio, GitHub, máquina local e deploy. A ordem importa: cada parte depende da anterior.

Os comandos mostram a forma crua (`aws ...`) para você aprender o que acontece. Na fatia 1 eles viram alvos do `Makefile` (`make deploy-foundation`, etc.). Templates e workflows citados aqui são criados durante a construção.

Convenções:

- `<owner>`: seu usuário do GitHub.
- Perfil AWS: `portfolio`. Região: `us-east-1`.

---

## Parte 1: Máquina local

| Ferramenta | Para quê | Verificar |
| --- | --- | --- |
| Go 1.25+ | API, export, testes | `go version` |
| Node 24 LTS + npm | React, Vite, Playwright | `node -v` |
| GNU Make | alvos do `Makefile` | `make --version` |
| Docker | MiniStack, AWS CLI local e cfn-lint (rodam em container) | `docker info` |
| AWS CLI v2 | deploy e acesso aos dados | `aws --version` |
| GitHub CLI | repo, rebuild | `gh --version` |
| golangci-lint | lint Go | `golangci-lint version` |
| mockgen (`go.uber.org/mock`) | mocks | `mockgen -version` |

Depois do primeiro `npm install` em `web/`:

```bash
npx playwright install --with-deps chromium
```

MiniStack: siga o quickstart em ministack.org (container Docker na porta `4566`). Para apontar CLI e SDKs para ele:

```bash
export AWS_ENDPOINT_URL=http://localhost:4566
```

---

## Parte 2: Conta AWS

### 2.1 Proteger a conta

1. Entre como root, ative **MFA** no root e não use mais o root no dia a dia.
2. Em **IAM Identity Center**, ative o serviço em `us-east-1`, crie um usuário para você e atribua o permission set `AdministratorAccess` à conta.
3. Configure o acesso na sua máquina (sem access key fixa):

```bash
aws configure sso --profile portfolio
aws sso login --profile portfolio
aws sts get-caller-identity --profile portfolio
```

### 2.2 Alertas de custo antes de tudo

Em **Billing and Cost Management → Billing preferences**, ative alertas de free tier (se houver) e o envio de faturas por e-mail. O Budget com alerta em US$ 2 é criado pela stack `foundation` (passo 2.4), mas não custa ativar isto antes.

### 2.3 Registrar o domínio

1. **Route 53 → Registered domains → Register domains**, busque `wendellopes.dev` (cerca de US$ 17/ano).
2. Deixe **privacy protection** ligado e **auto-renew** ligado.
3. O registro cria a **hosted zone** automaticamente (US$ 0,50/mês). Anote o ID:

```bash
aws route53 list-hosted-zones-by-name --dns-name wendellopes.dev --profile portfolio
```

Observação: `.dev` exige HTTPS em todo navegador (HSTS preload). Não é problema, o CloudFront com certificado ACM resolve.

### 2.4 Stack `foundation`

Cria o certificado ACM (validado sozinho via DNS na sua zona), o bucket de artefatos da Lambda, o provedor OIDC do GitHub, a role de deploy e o Budget.

```bash
aws cloudformation deploy \
  --profile portfolio --region us-east-1 \
  --stack-name portfolio-foundation \
  --template-file infra/foundation.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides \
      DomainName=wendellopes.dev \
      HostedZoneId=<id-da-zona> \
      GitHubRepo=<owner>/portfolio \
      BudgetEmail=wendel.lopes777@gmail.com
```

A validação do certificado pode levar alguns minutos; o comando espera. Se a conta já tiver um provedor OIDC do GitHub (de outro projeto), o template recebe o ARN existente por parâmetro em vez de criar outro.

### 2.5 Stack `data`

```bash
aws cloudformation deploy \
  --profile portfolio --region us-east-1 \
  --stack-name portfolio-data \
  --template-file infra/data.yaml
```

Tabela e bucket têm `DeletionPolicy: Retain`: apagar a stack não apaga seus Posts.

### 2.6 Stack `app` (fatia 5, contador)

A Lambda precisa do binário Go num bucket. O `package` faz o upload e reescreve o template:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o build/bootstrap ./cmd/api
aws cloudformation package \
  --profile portfolio \
  --template-file infra/app.yaml \
  --s3-bucket <bucket-de-artefatos> \
  --output-template-file build/app.packaged.yaml
aws cloudformation deploy \
  --profile portfolio --region us-east-1 \
  --stack-name portfolio-app \
  --template-file build/app.packaged.yaml \
  --capabilities CAPABILITY_IAM
```

### 2.7 Stack `edge`

CloudFront na frente do bucket (e da Lambda, quando a `app` existir) e registros DNS.

```bash
aws cloudformation deploy \
  --profile portfolio --region us-east-1 \
  --stack-name portfolio-edge \
  --template-file infra/edge.yaml \
  --parameter-overrides HostedZoneId=<id-da-zona>
```

A primeira criação de distribuição leva de 5 a 15 minutos.

### 2.8 Coletar outputs para o GitHub

```bash
for stack in foundation data app edge; do
  aws cloudformation describe-stacks --profile portfolio \
    --stack-name portfolio-$stack --query 'Stacks[0].Outputs' --output table
done
```

Anote: ARN da role de deploy, nome do bucket do Site, nome da tabela, ID da distribuição.

---

## Parte 3: GitHub

### 3.1 Repo

```bash
gh auth login
gh repo create <owner>/portfolio --public --source . --remote origin
```

### 3.2 Ruleset da `main`

**Settings → Rules → Rulesets → New branch ruleset**:

- Nome: `main`. Enforcement: **Active**. Target: branch padrão.
- Bypass list: **Repository admin** (você).
- Marque: **Restrict deletions**, **Block force pushes**, **Require a pull request before merging** (required approvals: 0), **Require status checks to pass** (adicione o job `ci` depois que ele rodar a primeira vez).

### 3.3 Actions

**Settings → Actions → General**:

- Fork PR workflows: **Require approval for all external contributors**.
- Workflow permissions: **Read repository contents and packages permissions**.
- Desmarque **Allow GitHub Actions to create and approve pull requests**.

### 3.4 Environment `production`

**Settings → Environments → New environment** `production`:

- Deployment branches and tags: **Selected branches** → `main`.
- Variables (não são segredos, por isso variables):

| Nome | Valor |
| --- | --- |
| `AWS_ROLE_ARN` | role de deploy (output da `foundation`) |
| `AWS_REGION` | `us-east-1` |
| `SITE_BUCKET` | bucket do Site (output da `data`) |
| `TABLE_NAME` | tabela (output da `data`) |
| `DISTRIBUTION_ID` | distribuição (output da `edge`) |

A role só confia em `repo:<owner>/portfolio:environment:production`. PR de fork não roda nesse environment, então não chega na AWS.

### 3.5 Workflows do Claude (opcional)

Se mantiver `claude.yml`/`claude-code-review.yml` do pricemonitor, adicione o segredo exigido pela action e confira que ambos têm a condição `author_association == 'OWNER'`.

---

## Parte 4: Primeiro deploy e dia a dia

### 4.1 Site no ar

Faça merge de um PR na `main`. O workflow `deploy.yml` assume a role via OIDC, roda `cmd/export`, gera o HTML, faz `aws s3 sync` e invalida o CloudFront. Para disparar à mão:

```bash
gh workflow run deploy.yml --ref main
gh run watch
```

Confira em `https://wendellopes.dev`.

### 4.2 Escrever e publicar um Post

```bash
aws sso login --profile portfolio
make painel
```

No Painel (localhost): criar Post → editar Versão → Publicar. O Painel grava no DynamoDB de produção e executa `gh workflow run deploy.yml`. Em poucos minutos o Post está no ar.

### 4.3 dev.to

**dev.to → Settings → Extensions → Publishing to DEV Community from RSS**:

- Feed URL: `https://wendellopes.dev/en/rss.xml`
- Marque **Mark the RSS source as canonical URL by default**.

Os Posts importados chegam como rascunho no dev.to; você revisa e publica lá.

### 4.4 Desenvolvimento local

```bash
make up            # sobe MiniStack
make deploy-local  # stacks data + app no MiniStack
make run           # API Go local (:8080)
make watch         # API Go com live reload (air)
make test          # unit + integração no MiniStack + coverage (precisa de up + deploy-local)
make test-unit     # só unit, sem MiniStack
make lint          # golangci-lint + cfn-lint
make painel        # Painel apontando para o MiniStack (ou prod, com AWS_PROFILE); chega na entrega 3
make e2e           # Playwright; chega na entrega 6
```

---

## Parte 5: Custo e manutenção

- O Budget avisa por e-mail se a previsão passar de US$ 2/mês.
- Conferir de vez em quando: **Billing → Bills**. Esperado: cerca de US$ 0,60/mês + domínio.
- Renovação do domínio: automática, anual.
- Sessão SSO expira; rode `aws sso login --profile portfolio` antes de usar o Painel.

## Parte 6: Desmontar (se um dia precisar)

Ordem inversa: `edge` → `app` → `data` → `foundation`.

```bash
aws cloudformation delete-stack --profile portfolio --stack-name portfolio-edge
```

A tabela e o bucket do Site ficam (Retain); apague à mão se quiser mesmo perder os dados. A zona do Route 53 e o domínio não são gerenciados pelas stacks.
