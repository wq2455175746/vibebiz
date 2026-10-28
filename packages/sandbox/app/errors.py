class WorkspaceError(Exception):
    status = 400
    code = 40000

    def __init__(self, message: str) -> None:
        super().__init__(message)
        self.message = message


class NotFoundError(WorkspaceError):
    status = 404
    code = 40400


class ConflictError(WorkspaceError):
    status = 409
    code = 40900


class TooLargeError(WorkspaceError):
    status = 413
    code = 41300
