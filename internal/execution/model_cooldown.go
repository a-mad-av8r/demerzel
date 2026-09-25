package execution

// UsesModelCooldown includes only operations that consume model reasoning quota; counting, probing, and resource management do not share the limit.
func (o Operation) UsesModelCooldown() bool {
	switch o {
	case OperationChatCompletion, OperationResponsesCreate, OperationResponsesCompact,
		OperationImagesGenerate, OperationImagesEdit, OperationEmbeddingsCreate, OperationRerank,
		OperationDecisionsCreate:
		return true
	default:
		return false
	}
}
