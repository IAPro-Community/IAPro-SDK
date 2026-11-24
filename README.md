# IAPro SDK

Bibliotecas de infraestrutura de IA compartilhadas por Nexus, Calango e futuros produtos IAPro. Produtos, dados, políticas de negócio e releases continuam independentes.

**Status:** pré-alpha, extração incremental. Não é uma release certificada nem uma aplicação executável.

## Objetivos

- Uma implementação canônica de contratos de providers, classificação de falhas, quotas, seleção e troca segura de contas, runtime, coordenação entre agentes e projeções AI Control/HUD.
- Reutilização com versões pinadas, testes de conformidade comuns e adapters de host.
- Sem dependência de Nexus ou Calango, sem serviço central obrigatório e sem compartilhamento implícito de credenciais ou banco.

## Plano

- [Nexus #235](https://github.com/IAPro-Community/nexus/issues/235): extração e integração, Lane 03.
- [Calango #128](https://github.com/IAPro-Community/calango/issues/128): consumo incremental.
- Consulte a documentação de arquitetura e procedência para limites e gates antes de publicar uma versão.

A base inicial é deliberadamente pequena. Módulos não implementados não devem ser anunciados como disponíveis.

## Implementação atual (pré-alpha)

| Pacote | Escopo |
|---|---|
| model / classifier / quota | Tipos, classificação de falhas e elegibilidade |
| providers/spec | Contratos de seis CLIs e goldens; metadados de flags não são autorização de execução |
| providers/parsers | Normalização de respostas Claude/Gemini já obtidas pelo host |
| runtime | Descoberta de binários e ambiente, sem executar providers |
| routing | Motivo tipado e gate conservador de fallback; sem dispatch ou handoff |
| coordination | Envelope de coordenação, validação e ID semântico; host controla autorização e persistência |
| capacity | Thresholds, transições e fingerprint |
| hud | DTOs e seleção de foco; sem widget ou probe |

`go test ./...`, `go vet ./...`. Não utilizar flags de bypass de aprovação extraídas dos contratos como política de execução. Autorização, leases, orçamento e segurança permanecem no host. Adapters completos, transações de handoff, integração dos consumidores e UI ainda são gates em aberto.
