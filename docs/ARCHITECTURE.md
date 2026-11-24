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
