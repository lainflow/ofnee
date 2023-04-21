from function_framework import FunctionContext
from datetime import datetime

def user_function(context: FunctionContext):
    print(context.request.metadata, flush=True)
    print(context.request.text(), flush=True)

    now = datetime.now()

    current_time = now.strftime("%H:%M:%S")
    print("Current Time =", current_time)

    # 用户函数处理逻辑
    output_data = "hello!!!!!!"
    return output_data