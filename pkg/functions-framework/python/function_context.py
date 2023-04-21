from dapr.ext.grpc import App, InvokeMethodRequest, InvokeMethodResponse
from dapr.clients import DaprClient


class FunctionContext():
    def __init__(self, name, port) -> None:
        self.name = name
        self.port = port
        self.dapr_client = DaprClient
        self.app = App()
        self.user_function = None
        self.request = None

    def load_user_function(self, user_function):
        self.user_function = user_function

    def invoke_user_function(self, request):
        self.request = request
        if self.user_function:
            output_data = self.user_function(self)
            return output_data
        else:
            raise ValueError("User function is not loaded.")

    def run(self):
        @self.app.method(name=self.name)
        def function_method(request: InvokeMethodRequest) -> InvokeMethodResponse:
            output_data = self.invoke_user_function(request)
            return InvokeMethodResponse(output_data.encode(), "text/plain; charset=UTF-8")

        self.app.run(self.port)
