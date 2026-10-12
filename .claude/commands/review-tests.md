---
description: Revisa qualidade de testes, BDD, cobertura de cenários e resistência a regressão com base nas regras e categorias definidas na skill `test-review`.
subtask: true
---

Realize uma revisão completa de qualidade de testes no path: $ARGUMENTS

Se nenhum path foi fornecido, identifique os arquivos de teste Go alterados no branch atual comparado com a branch principal:
!`git diff --name-only $(git merge-base HEAD main)..HEAD -- '*_test.go' | head -30`

## Instruções

1. Carregue a skill `test-review` antes de iniciar a análise.
2. Siga o workflow de exploração definido na skill para mapear a estrutura de testes.
3. Para cada arquivo `_test.go` no escopo, leia também o arquivo de produção correspondente para entender quais cenários deveriam ser cobertos.
4. Execute a detecção em todas as categorias.
5. Verifique também as regras específicas de CLAUDE.md (seção Testes).
6. Se os testes estão bem estruturados, retorne status `pass` com um resumo positivo.

## Contexto do projeto

Consulte @CLAUDE.md para convenções específicas deste projeto.
