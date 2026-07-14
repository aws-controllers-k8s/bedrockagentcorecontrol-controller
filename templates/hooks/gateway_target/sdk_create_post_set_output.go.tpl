if desired.ko.Spec.TargetConfiguration != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Lambda != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload != nil {
    ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload = desired.ko.Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload
}
if desired.ko.Spec.TargetConfiguration != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp != nil &&
    desired.ko.Spec.TargetConfiguration.Mcp.Connector != nil &&
    ko.Spec.TargetConfiguration != nil &&
    ko.Spec.TargetConfiguration.Mcp != nil &&
    ko.Spec.TargetConfiguration.Mcp.Connector != nil {
    ko.Spec.TargetConfiguration.Mcp.Connector.Configurations = desired.ko.Spec.TargetConfiguration.Mcp.Connector.Configurations
}
