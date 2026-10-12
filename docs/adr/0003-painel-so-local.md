# Painel roda só na máquina do Wendel

O Painel e as rotas de escrita da API existem no repo, mas não são publicados. Rodam localmente com o perfil AWS do Wendel, escrevendo direto no DynamoDB de produção, e disparam o rebuild com `gh workflow run`. Na nuvem, a Lambda expõe só o contador de Visualizações.

Publicar o Painel exigiria Cognito, validação de JWT, rotas admin expostas, token do GitHub guardado na AWS e uma stack a mais, tudo para um único autor. Abrimos mão de escrever Posts fora do próprio computador em troca de menos superfície de ataque e cerca de 40% menos peças.

## Consequences

- Levar o Painel para a nuvem depois é aditivo: as rotas admin já existem, faltam auth e exposição.
- Credenciais AWS locais (SSO) passam a ser o controle de acesso de escrita.
