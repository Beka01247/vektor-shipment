# Load the restart_process extension
load('ext://restart_process', 'docker_build_with_restart')

### K8s Config ###

k8s_yaml('./infra/development/k8s/app-config.yaml')

### End of K8s Config ###

### Shipment Service ###

shipment_compile_cmd = 'cd services/shipment-service && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../build/shipment-service ./cmd/main.go'
if os.name == 'nt':
  shipment_compile_cmd = './infra/development/docker/shipment-build.bat'

local_resource(
  'shipment-service-compile',
  shipment_compile_cmd,
  deps=['./services/shipment-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'vektor-shipment/shipment-service',
  '.',
  entrypoint=['/app/build/shipment-service'],
  dockerfile='./infra/development/docker/shipment-service.Dockerfile',
  only=[
    './build/shipment-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_yaml('./infra/development/k8s/shipment-service-deployment.yaml')
k8s_resource('shipment-service', 
             port_forwards=50052,
             resource_deps=['shipment-service-compile'], 
             labels="services")
k8s_resource('postgres', 
             port_forwards=5432,
             labels="infrastructure")
k8s_resource('rabbitmq', 
             port_forwards=['5672', '15672:15672'],
             labels="infrastructure")

### End of Shipment Service ###
