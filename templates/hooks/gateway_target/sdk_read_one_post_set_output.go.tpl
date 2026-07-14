if err := setSchemaDefinitionsFromSDKResponse(ko, resp); err != nil {
    return nil, err
}
if err := setConnectorParameterValuesFromSDKResponse(ko, resp); err != nil {
    return nil, err
}