schema Deployment:
    name: str
    replicas: int = 1
    image: str

deployment = Deployment {
    name = "demo"
    replicas = 3
    image = "nginx:1.27"
}
