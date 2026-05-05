if desired.ko.Spec.TargetConfiguration != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Lambda != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload != nil {
    ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload = desired.ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload
}