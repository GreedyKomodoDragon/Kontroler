export POSTGRES_PASSWORD=$(kubectl get secret --namespace default postgres-postgresql -o jsonpath="{.data.postgres-password}" | base64 -d)
export SSL_MODE=require  # Change this to 'disable', 'require', 'verify-ca', or 'verify-full' as needed

kubectl run postgres-postgresql-client --rm --tty -i --restart='Never' --namespace default --image docker.io/bitnami/postgresql:16.4.0-debian-12-r7 \
      --env="PGPASSWORD=$POSTGRES_PASSWORD" --env="PGSSLMODE=$SSL_MODE" \
      --command -- psql --host postgres-postgresql -U postgres -d postgres -p 5432 --set=sslmode=$SSL_MODE
