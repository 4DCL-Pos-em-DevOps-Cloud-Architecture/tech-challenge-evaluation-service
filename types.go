package main

import "fmt"

// --- Estruturas de Dados ---

// Flag espelha a resposta do flag-service
type Flag struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsEnabled   bool   `json:"is_enabled"`
}

// TargetingRule espelha a resposta do targeting-service
type TargetingRule struct {
	ID         int    `json:"id"`
	FlagName   string `json:"flag_name"`
	IsEnabled  bool   `json:"is_enabled"`
	Rules      Rule   `json:"rules"` // O objeto JSONB
}

// Rule é o objeto JSONB aninhado
type Rule struct {
	Type  string      `json:"type"`  // ex: "PERCENTAGE"
	Value interface{} `json:"value"` // ex: 50
}

// CombinedFlagInfo é a estrutura que salvamos no cache
type CombinedFlagInfo struct {
	Flag *Flag
	Rule *TargetingRule
}

// NotFoundError é um erro customizado
type NotFoundError struct {
	FlagName string
}
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("flag ou regra '%s' não encontrada", e.FlagName)
}