CREATE TABLE public.stream (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT stream_pkey PRIMARY KEY (id ASC)
);
CREATE TABLE public.webhook (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT webhook_pkey PRIMARY KEY (id ASC),
	INVERTED INDEX webhook_events ((data->'events':::STRING)),
	INDEX "webhook_userId" ((data->>'userId':::STRING) ASC)
);
CREATE TABLE public.session (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT session_pkey PRIMARY KEY (id ASC)
);
CREATE TABLE public.asset (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT asset_pkey PRIMARY KEY (id ASC),
	INDEX asset_id ((data->>'id':::STRING) ASC),
	INDEX "asset_playbackId" ((data->>'playbackId':::STRING) ASC),
	INDEX asset_source_url (((data->'source':::STRING)->>'url':::STRING) ASC),
	INDEX "asset_source_sessionId" (((data->'source':::STRING)->>'sessionId':::STRING) ASC),
	INDEX "asset_storage_ipfs_nftMetadata_cid" (((((data->'storage':::STRING)->'ipfs':::STRING)->'nftMetadata':::STRING)->>'cid':::STRING) ASC),
	INDEX asset_storage_ipfs_cid ((((data->'storage':::STRING)->'ipfs':::STRING)->>'cid':::STRING) ASC),
	INDEX "asset_userId" ((data->>'userId':::STRING) ASC),
	INDEX "asset_playbackRecordingId" ((data->>'playbackRecordingId':::STRING) ASC),
	INDEX "asset_sourceAssetId" ((data->>'sourceAssetId':::STRING) ASC)
);
CREATE TABLE public.users (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT users_pkey PRIMARY KEY (id ASC),
	UNIQUE INDEX users_email ((data->>'email':::STRING) ASC),
	UNIQUE INDEX "users_stripeCustomerId" ((data->>'stripeCustomerId':::STRING) ASC)
);
CREATE TABLE public.webhook_response (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT webhook_response_pkey PRIMARY KEY (id ASC),
	INDEX "webhook_response_webhookId" ((data->>'webhookId':::STRING) ASC),
	INDEX "webhook_response_eventId" ((data->>'eventId':::STRING) ASC)
);
CREATE TABLE public.multistream_target (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT multistream_target_pkey PRIMARY KEY (id ASC),
	INDEX "multistream_target_userId" ((data->>'userId':::STRING) ASC)
);
CREATE TABLE public.task (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT task_pkey PRIMARY KEY (id ASC),
	INDEX task_id ((data->>'id':::STRING) ASC),
	INDEX "task_inputAssetId" ((data->>'inputAssetId':::STRING) ASC),
	INDEX "task_outputAssetId" ((data->>'outputAssetId':::STRING) ASC),
	INDEX "task_userId" ((data->>'userId':::STRING) ASC),
	INDEX "task_pendingTasks" ((data->>'userId':::STRING) ASC, (COALESCE(((data->'status':::STRING)->>'updatedAt':::STRING)::INT8, 0:::INT8)) ASC, ((data->'status':::STRING)->>'phase':::STRING) ASC)
);
CREATE TABLE public.usage (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT usage_pkey PRIMARY KEY (id ASC),
	UNIQUE INDEX usage_id ((data->>'id':::STRING) ASC)
);
CREATE TABLE public.signing_key (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT signing_key_pkey PRIMARY KEY (id ASC),
	INDEX signing_key_id ((data->>'id':::STRING) ASC),
	INDEX "signing_key_userId" ((data->>'userId':::STRING) ASC)
);
CREATE TABLE public.room (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT room_pkey PRIMARY KEY (id ASC),
	INDEX "room_userId" ((data->>'userId':::STRING) ASC)
);
CREATE TABLE public.attestation (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT attestation_pkey PRIMARY KEY (id ASC)
);
CREATE TABLE public.object_store (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT object_store_pkey PRIMARY KEY (id ASC),
	INDEX "object_store_userId" ((data->>'userId':::STRING) ASC)
);
CREATE TABLE public.password_reset_token (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT password_reset_token_pkey PRIMARY KEY (id ASC),
	INDEX "password_reset_token_userId" ((data->>'userId':::STRING) ASC)
);
CREATE TABLE public.experiment (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT experiment_pkey PRIMARY KEY (id ASC),
	UNIQUE INDEX experiment_name ((data->>'name':::STRING) ASC),
	INDEX "experiment_userId" ((data->>'userId':::STRING) ASC),
	INVERTED INDEX "experiment_audienceUserIds" ((data->'audienceUserIds':::STRING))
);
CREATE TABLE public.regions (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT regions_pkey PRIMARY KEY (id ASC),
	UNIQUE INDEX regions_region ((data->>'region':::STRING) ASC)
);
CREATE TABLE public.api_token (
	id VARCHAR(128) NOT NULL,
	data JSONB NULL,
	CONSTRAINT api_token_pkey PRIMARY KEY (id ASC),
	INDEX "api_token_userId" ((data->>'userId':::STRING) ASC)
);
