# Posts no DynamoDB, servidos como HTML estático reconstruído ao publicar

Posts são escritos no Painel e guardados no DynamoDB, não como Markdown versionado no repo. Ao publicar, um workflow do GitHub Actions (disparado por `gh workflow run`) lê os Posts, gera HTML estático (React SSG), envia ao S3 e invalida o CloudFront. Visitantes nunca chamam o backend para ler um Post; só o contador de Visualizações chama a API.

Escolhido para estudar DynamoDB e manter um Painel embutido, sem abrir mão de SEO, preview em redes sociais e custo de leitura perto de zero.

## Considered Options

- **Markdown no repo**: mais simples e sem backend, mas sem Painel e sem DynamoDB para estudar.
- **Página busca o Post na API ao abrir**: dispensa rebuild, mas entrega HTML vazio a crawlers e ao preview do LinkedIn, e cada leitura custa uma invocação.

## Consequences

- Publicar ou editar um Post só aparece no Site depois do rebuild (minutos, não segundos).
- O build depende de credenciais AWS (OIDC) para ler o DynamoDB.
