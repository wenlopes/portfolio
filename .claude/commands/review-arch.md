---
description: Revisa arquitetura, boundaries e princípios de um módulo ou path com base nas regras e categorias definidas na skill `arch-review`.
subtask: true
---

Realize uma revisão arquitetural completa no path: $ARGUMENTS

Se nenhum path foi fornecido, identifique os arquivos Go alterados no branch atual comparado com a branch principal:
!`git diff --name-only $(git merge-base HEAD main)..HEAD -- '*.go' | grep -v '_test.go' | grep -v '/mocks/' | head -30`

## Instruções

1. Carregue a skill `arch-review` antes de iniciar a análise.
2. Siga o workflow de exploração definido na skill para mapear a estrutura.
3. Para cada arquivo `.go` no escopo, leia também os arquivos relacionados (testes, interfaces, implementações, subpacotes) para entender o contexto completo do módulo.
4. Execute a detecção em todas as categorias.
5. Verifique também as regras específicas de CLAUDE.md.
6. Se o módulo está bem estruturado, retorne status `pass` com um resumo positivo.

## Contexto do projeto

Consulte @CLAUDE.md para convenções específicas deste projeto.
