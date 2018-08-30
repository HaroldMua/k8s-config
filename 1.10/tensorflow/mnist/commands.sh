PS_HOSTS="ps0.tf-service.default.svc.cluster.local:2222"
WORKER_HOSTS="worker0.tf-service.default.svc.cluster.local:2222,worker1.tf-service.default.svc.cluster.local:2222,worker2.tf-service.default.svc.cluster.local:2222"

# On ps0
python trainer.py \
--ps_hosts=ps0.tf-service.default.svc.cluster.local:2222 \
--worker_hosts=worker2.tf-service.default.svc.cluster.local:2222 \
--job_name=ps --task_index=$TASK_INDEX
# On worker0
python trainer.py \
--ps_hosts=ps0.tf-service.default.svc.cluster.local:2222 \
--worker_hosts=worker0.tf-service.default.svc.cluster.local:2222,worker1.tf-service.default.svc.cluster.local:2222,worker2.tf-service.default.svc.cluster.local:2222 \
--job_name=worker --task_index=$TASK_INDEX