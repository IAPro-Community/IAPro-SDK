# Changelog

## Unreleased

- Inicialização do módulo Go e extração de tipos, classificação de falhas e projeção/gates de quota do Nexus.
- Testes de caracterização portados e regressões de quota desconhecida/grupos independentes.
- Sem tag, sem publicação de pacote e sem integração em consumidores.

- Extração de provider specs e goldens, descoberta de executáveis, parsing de quotas Claude/Gemini, coordenação tipada, transições de capacidade e modelos HUD.
- Gate de fallback conservador com testes; não executa handoff nem transfere sessões.

- Corrigir fixture OpenCode: revisão nativa continua não verificada, sem flag obsoleta.
- Classificação HTTP tipada e gate de fallback de cadeia explícita para consumidores.
- CI publica metadados Go da revisão imutável para integração dos consumidores.

- Endurecer fallback: identidade da evidência de quota, grupo/modelo explícitos, janelas válidas e proteção contra overflow no retry.

- Rejeitar percentuais ausentes ou inválidos, proteger specs de mutação, exigir ausência de efeitos após reconciliação e clonar timestamps de coordenação.
- Adicionar contrato neutro de adapters e normalização do resultado de quota Codex com identidade e bloqueio oficial.

- Adicionar cache de quota isolado, runtime Codex JSONL e RPC, transições de handoff, lifecycle de coordenação, fencing por CAS, projeção de capacidade e evidências de custo.
- Adicionar componentes React read-only e testes de renderização, com lockfile e typecheck.

- Consolidar execução estruturada em providers/process e adicionar decoder/runtime Gemini stream-json com fixtures, terminal obrigatório e contadores opcionais.

- Compartilhar framing JSONL bounded/sticky e adicionar runtime Claude Code para sessão nova single-shot, uso agregado por modelo e fixtures.
