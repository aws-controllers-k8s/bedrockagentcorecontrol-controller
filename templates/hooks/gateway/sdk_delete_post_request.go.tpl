if err != nil {
    var awsErr smithy.APIError
    if errors.As(err, &awsErr) && awsErr.ErrorCode() == "ValidationException" && strings.HasSuffix(awsErr.ErrorMessage(), "Delete all targets before deleting the gateway.") {
        return nil, requeueGateTargetDeleting
    }
}