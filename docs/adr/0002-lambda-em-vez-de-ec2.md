# Lambda atrás do CloudFront em vez de EC2

O backend em Go roda numa única Lambda (router `net/http`, Function URL) atrás do CloudFront, sem API Gateway. Sem contar free tier, o custo estimado é cerca de US$ 0,60/mês para o Site inteiro, contra cerca de US$ 8/mês de uma EC2 t4g.nano (instância + IPv4 público + EBS), que ainda exigiria patches de SO. O teto de custo do projeto é o fator decisivo.

## Considered Options

- **EC2**: modelo mental mais simples, mas cerca de 13x mais caro e com manutenção de servidor.
- **Lambda por rota + API Gateway**: mais templates, mais cold starts e custo de API Gateway sem ganho nesta escala.
