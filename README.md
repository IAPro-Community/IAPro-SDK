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
| providers/parsers | Normalização de respostas Claude/Gemini/Codex já obtidas pelo host |
| providers | Contratos de adapters, cache de quota com escopo de identidade, execução Codex/Gemini e transporte RPC Codex com launch autorizado |
| runtime | Descoberta de binários e ambiente com isolamento de variáveis Windows |
| routing | Gate de fallback e transições de handoff; execução, CAS e evidência pertencem ao host |
| coordination | Envelope, admissão idempotente, limites, lineage e resolução; host persiste sob transação |
| capacity | Thresholds, projeção de observações por identidade e transições |
| hud | DTOs e seleção de foco, sem probe |
| cost | Resumo de evidências de custo e validação de orçamento consultiva |
| lease | Coordenação por CAS/fencing; store em memória para testes, store durável a cargo do host |
| ui/react | AI Control, ProviderCard, CostPanel e HUD read-only; pacote privado até revisão de release |

`go test ./...`, `go vet ./...`. Não utilizar flags de bypass de aprovação extraídas dos contratos como política de execução. Autorização, leases, orçamento e segurança permanecem no host. Adapters completos, transações de handoff, integração dos consumidores e UI ainda são gates em aberto.

## Matriz de evidência (não é certificação de produção)

| Provider | Specs/discovery | Parser quota | Execução estruturada | Auth/refresh e sessão real |
|---|---|---|---|---|
| Codex | fixtures | parser e RPC via host | runtime JSONL e fixture subprocesso | login status + atribuição local; homologação pendente |
| Claude | fixtures | parser | pendente | pendente |
| Gemini | fixtures | parser | runtime stream-json e fixture subprocesso | pendente |
| AGY | fixtures | pendente | pendente | pendente |
| OpenCode | fixtures | não comprovado | pendente | pendente |
| Cursor | fixtures | não comprovado | pendente | pendente |

A integração do Calango requer versão pinada com checksums, CI e revisão do host; não anunciar SDK estável nem ativar handoff automático até validar persistência, credenciais e efeitos reais. A migração do Nexus foi explicitamente adiada pelo mantenedor.
