FROM python:3.12-slim

RUN pip install --no-cache-dir pytest requests paramiko psycopg2-binary

WORKDIR /tests
COPY . /tests/

# 默认等待环境就绪后执行测试
ENTRYPOINT ["python3", "-m", "pytest", "test_e2e_docker.py", "-v", "--tb=short"]
