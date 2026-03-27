package services

import "neo-code/internal/agentruntime/interaction"

type ApprovalDecision = interaction.ApprovalDecision

const (
	ApprovalDecisionApprove = interaction.ApprovalDecisionApprove
	ApprovalDecisionReject  = interaction.ApprovalDecisionReject
)

type Controller = interaction.Controller
type BootstrapData = interaction.BootstrapData
type ConversationRequest = interaction.ConversationRequest
type TurnResolution = interaction.TurnResolution

func NewRuntimeController(client ChatClient, configPath string) Controller {
	return interaction.NewRuntimeController(client, configPath)
}
