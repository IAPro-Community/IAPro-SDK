# Arquitetura e fronteiras

Status: pré-alpha. O SDK é biblioteca, não um segundo control plane. Nexus e Calango mantêm seus próprios bancos, runtime de domínio, autenticação de usuários, orçamento, aprovações, UX e releases.

## Implementado neste recorte

- `model`: contratos neutros de identidade de conta, provider, uso, capabilities e falhas.
- `classifier`: classificação e intervalo de retry extraídos do Nexus, sem executar ações.
- `quota`: projeção de snapshots já obtidos pelo host e gates de elegibilidade. Não consulta credenciais ou rede. Seleção de modelo, frescor, autorização, orçamento e efeitos externos precisam de validação pelo host. Estados UNKNOWN e UNSUPPORTED não são permissão automática. Preservamos comportamento upstream para caracterização; revisar casos de status ERROR, janelas expiradas, grupos/modelos e cooldown antes de integrar em decisões automáticas.

## Próximas fases

S1 provider specs e discovery (desacoplar runtime); S2 contas, probes, escopo e cache; S3 routing/failover, cancelamento, leases e idempotência; S4 comunicação tipada entre agentes; S5 capacity/cost/HUD e React; S6 coordenação opcional entre processos. Ver Nexus #235 e Calango #128.

Troca de contas exige autorização explícita, termos do provider, isolamento, identidade verificada e ausência de side effects ambíguos. Não retomar sessão entre contas sem capacidade demonstrada. SDK não deve depender dos produtos. Delegação não concede privilégios, não inicia agentes diretamente e não replica MissionRunner. HUD consome read models, nunca faz probe. Bibliotecas isoladas não impõem limites globais entre processos.

## Antidivergência

Mover código, fixtures e testes como lote. Publicar versões imutáveis após aprovação jurídica e CI; consumidores pinam módulo e validam a mesma suite. Manter wrappers temporários com prazo de retirada e rollback. Não usar replace local em releases. Nenhuma remoção de código no consumidor sem paridade. Não usar dependência AGPL em biblioteca MIT sem autorização específica. Sem release automática enquanto gates legais, de qualidade e de integração estiverem abertos.

## Gates

`go test ./...`, `go test -race ./...` (ambiente com CGO), `go vet ./...`, `gofmt`; Linux/Windows/macOS. Testes de provider real somente autorizados. Gates de consumidores separados; não modificar releases em estabilização. Ver NOTICE e Nexus #74 para revisão jurídica. Versão Go 1.23 inicial por compatibilidade; upgrade depende de ADR.

## Segundo recorte

Provider specs e runtime discovery, parsing de quota Claude/Gemini, envelopes de coordenação, threshold de capacidade, motivo/gate de fallback e DTOs/seleção de foco HUD. Os contratos de CLI preservam o inventário histórico de flags; não são recomendações de segurança nem autorização para execução. O host deve implementar isolamento, secret store e integração com APIs suportadas. Não há execução de probes nem transferência de sessão neste lote. O gate de fallback exige conta distinta autorizada, termos permitidos, orçamento, evidência de quota fresca e fronteira segura. O host revalida e reserva antes de iniciar. Coordenação preserva formato Nexus para adapters; status e IDs são controlados pelo host, não pelo modelo. O SDK não materializa tarefas. HUD é projeção read-only; UI e tipos específicos de produto ainda exigem adapters.

## Revisão de segurança

SelectFallback exige QuotaScope derivado da observação original, e não copiado da identidade atual; deve coincidir com Scope integralmente. Model e GroupMapped são explícitos, QuotaGroup corresponde ao pool do modelo. Janelas desconhecidas/expiradas não autorizam seleção. O host é responsável por obter e vincular evidência autêntica, lease e execução. Classify recebe somente diagnósticos confiáveis, nunca texto gerado pelo modelo.

## Contratos de adapters e gate de execução

O pacote providers define portas neutras de descoberta, autenticação, quota, execução tipada e continuidade. Ainda não há implementação de subprocesso ou autenticação real nesses contratos. O parser Codex aceita somente o resultado de account/rateLimits/read, após o host comprovar home isolado e identidade externa; falta integrar transporte oficial e locks. Não confundir fixtures com homologação real.

## Recorte de implementação S2–S6

Codex: `providers/codex.RunExec` executa um plano de launch autorizado, stdin e stdout JSONL, sem retransmitir stderr ou executar retry. O decode emite mensagens de agente completas (não deltas de tokens), mede uso apenas quando turn.completed fornece ambos contadores e exige sucesso do processo. `StartStdioRPC` é transporte transiente; o host adquire lock de manutenção, cria processo com ambiente isolado e comprova identidade antes/depois. Protocolo quota-only em providers/parsers/codex. Ainda não há implementação real de auth/refresh e locks para todos os providers.

`UsageCache` deduplica em processo, TTL/backoff, escopo completo de owner/identidade e revalidação de autenticação; o host deve obter identidade e geração confiáveis. `routing.AdvanceHandoff` é transição pura, persistência e efeitos idempotentes via host. `lease.Coordinator` exige CAS durável e fencing no ponto de escrita, não toma recurso expirado sem prova de quiescência; MemoryStore não coordena processos. `coordination.Admit/Resolve` exige autorização, run/turn/lineage e transação host. `capacity.Observe` projeta observações; scheduler e entrega de alertas permanecem do host. `cost.Summarize` mantém estimado/medido/unknown separados e rejeita duplicidade/overflow. UI React consome projeções, sem probes/routing. Testes herméticos não são certificação de provider real.

Referência pública para Codex exec JSONL: https://developers.openai.com/blog/eval-skills. Nenhuma mudança no Nexus neste recorte.

O fallback pago agora exige reserva do ledger vinculada ao owner, run, escopo, modelo, valor e expiração; o host deve consumir a reserva atomicamente antes do dispatch. O QuotaView transporta identidade e expiração originais do snapshot. Sem esses campos o fallback automático é inelegível.

## Runtime estruturado compartilhado

`providers/process.Run` centraliza stdin, cancelamento, execução e sanitização de falhas. Codex e Gemini usam o mesmo runner, com decoders independentes. Gemini usa `--output-format stream-json` (https://geminicli.com/docs/cli/headless/), emite apenas mensagens de assistente e exige result.status=success, sessão e saída, além do exit code zero. Contadores ausentes/inválidos continuam desconhecidos. Fixture de subprocesso não certifica versão ou autenticação real. O host precisa validar launch e política; não habilitar provider automaticamente.
