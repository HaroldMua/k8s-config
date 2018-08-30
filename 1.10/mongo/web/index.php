<?php
	$mongoClient = new MongoDB\Driver\Manager('mongodb://mongod-0.mongodb-service.default.svc.cluster.local:27017,mongod-1.mongodb-service.default.svc.cluster.local:27017');
	$filter = [];
	$options = [];
	$query = new MongoDB\Driver\Query($filter, $options);
	$cursor = $mongoClient->executeQuery('test.users', $query);
	
	foreach ( $cursor as $data ){
		echo($data->name.' ');
	}
	
?>

