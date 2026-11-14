if err := setSchemaDefinitionsOnUpdateInput(desired, input); err != nil {
    return nil, err
}
if err := setConnectorParameterValuesOnUpdateInput(desired, input); err != nil {
    return nil, err
}