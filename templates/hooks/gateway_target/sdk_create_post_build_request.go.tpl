if err := setSchemaDefinitionsOnCreateInput(desired, input); err != nil {
    return nil, err
}
if err := setConnectorParameterValuesOnCreateInput(desired, input); err != nil {
    return nil, err
}