package llm

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/andrey/portfolio-reports/internal/models"
)

const fallbackTemplate = `Краткое резюме:
Адрес {{ .Address }} управляет портфелем на {{ formatUSD .TotalValue }}. В составе {{ .Diversification.TokenCount }} токенов. Уровень риска: {{ .Risk.Level }}.

Состав портфеля:
{{- range $i, $t := .Tokens }}
{{- if lt $i 6 }}
- {{ $t.Symbol }} ({{ $t.Mint }}): {{ formatUSD $t.ValueUSD }}, доля {{ formatPct $t.PctOfPortfolio }}{{ if $t.IsStablecoin }}, стейблкоин{{ end }}{{ if eq $t.RiskCategory "scam" }}, помечен как скам{{ end }}.
{{- end }}
{{- end }}

Доходность:
Реализованная: {{ formatUSD .PnL.Realized }}. Нереализованная: {{ formatUSD .PnL.Unrealized }}. Итого PnL: {{ formatUSD .PnL.Total }} ({{ formatPct .PnL.PctReturn }} от вложений).

Диверсификация:
Индекс Херфиндаля: {{ formatNum .Diversification.HerfindahlIndex }} (уровень концентрации: {{ .Diversification.ConcentrationLevel }}). Доля стейблкоинов: {{ formatPct .Diversification.StablecoinPct }}. Доля топ-токена: {{ formatPct .Diversification.TopTokenPct }}.

Риски:
Скор риска: {{ formatNum .Risk.Score }}/100. Доля волатильных активов: {{ formatPct .Risk.VolatilePct }}. Подозрительных токенов: {{ formatPct .Risk.ScamTokenPct }}.
{{- range .Risk.Factors }}
- {{ . }}
{{- end }}

Рекомендации:
{{ recommendations . }}`

const summaryTemplate = `Портфель на {{ formatUSD .TotalValue }} из {{ .Diversification.TokenCount }} токенов, риск {{ .Risk.Level }}, PnL {{ formatUSD .PnL.Total }}.`

type FallbackRenderer struct {
	report  *template.Template
	summary *template.Template
}

func NewFallbackRenderer() (*FallbackRenderer, error) {
	funcs := template.FuncMap{
		"formatUSD":      formatUSD,
		"formatPct":      formatPct,
		"formatNum":      formatNum,
		"recommendations": recommendations,
	}
	r, err := template.New("report").Funcs(funcs).Parse(fallbackTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}
	s, err := template.New("summary").Funcs(funcs).Parse(summaryTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse summary: %w", err)
	}
	return &FallbackRenderer{report: r, summary: s}, nil
}

func (r *FallbackRenderer) Render(p *models.Portfolio) (text string, summary string, err error) {
	var rb, sb bytes.Buffer
	if err := r.report.Execute(&rb, p); err != nil {
		return "", "", fmt.Errorf("execute report: %w", err)
	}
	if err := r.summary.Execute(&sb, p); err != nil {
		return "", "", fmt.Errorf("execute summary: %w", err)
	}
	return rb.String(), sb.String(), nil
}

func formatUSD(v float64) string  { return fmt.Sprintf("$%.2f", v) }
func formatPct(v float64) string  { return fmt.Sprintf("%.1f%%", v) }
func formatNum(v float64) string  { return fmt.Sprintf("%.3f", v) }

func recommendations(p *models.Portfolio) string {
	var out []string
	switch p.Risk.Level {
	case models.RiskCritical:
		out = append(out, "Сократить долю подозрительных токенов и пересмотреть весь портфель.")
	case models.RiskHigh:
		out = append(out, "Увеличить долю стейблкоинов до 20-30 процентов для защиты.")
	case models.RiskMedium:
		out = append(out, "Сохранять текущую структуру, отслеживать выход из волатильных позиций.")
	default:
		out = append(out, "Состояние портфеля стабильное.")
	}
	if p.Diversification.ConcentrationLevel == models.ConcentrationHigh {
		out = append(out, "Распределить капитал между 8-10 разными активами.")
	}
	if p.Diversification.StablecoinPct < 10 {
		out = append(out, "Добавить часть капитала в стейблкоины как буфер.")
	}
	return strings.Join(out, " ")
}
