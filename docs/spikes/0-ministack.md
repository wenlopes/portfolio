# Spike 0: MiniStack

**Data:** 2026-10-12
**Resultado:** viável. As stacks `data` e `app` sobem via CloudFormation no MiniStack e o repository Go grava e lê.

## Como reproduzir

```bash
make up
make deploy-local
make test-integration
```

## Achados

- Imagem `ministackorg/ministack`, porta `4566`, credenciais `test`/`test`, `us-east-1`. Sobe em poucos segundos; estado só em memória (`make down` apaga tudo).
- CloudFormation: `deploy`, `package` (upload do zip para bucket local) e `Fn::ImportValue` entre stacks funcionam. Nomes físicos seguem o padrão da AWS (`portfolio-data-Table-<hash>`), então o nome da tabela vem do output da stack.
- DynamoDB cobre os padrões de acesso 2, 3 e 4 da spec: `TransactWriteItems` com `ConditionCheck` e `CancellationReasons`, `Query` no GSI1 com `ScanIndexForward=false`, `UpdateItem ADD` com condição e `ReturnValues`.
- O padrão 1 (`Scan` do Painel) não foi exercitado; fica para a entrega 3.
- O bucket de artefatos é da stack `foundation`, que não roda no MiniStack; `make deploy-local` cria um bucket local à parte.
- Decisões tomadas além da spec: `Retain` também na tabela (proteger Posts) e `IncrementViews` com `attribute_exists(PK)`, para uma Visualização não criar Post fantasma.
- Lambda: runtime `provided.al2023`, `arm64`, variáveis de ambiente, Function URL e log group com retenção de 14 dias são criados.
- `ReservedConcurrentExecutions` é aceito no template mas `get-function-concurrency` volta vazio. Não dá para validar o limite localmente; conferir na AWS na entrega 5.
- Invocação da Lambda não foi testada no spike (o handler ainda não tem rotas).
- AWS CLI roda via imagem `amazon/aws-cli`, sem instalação local. A máquina não tem `make`, `golangci-lint` nem `cfn-lint` instalados (ver `docs/setup.md`, Parte 1).
