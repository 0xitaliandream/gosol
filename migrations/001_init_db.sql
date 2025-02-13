CREATE TABLE wallets (
    id UUID DEFAULT generateUUIDv4(),
    address String,
    is_lower_bound_synced Bool DEFAULT false,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (id);

CREATE TABLE transactions (
    id UUID DEFAULT generateUUIDv4(),
    sequence Int64,
    wallet_id UUID, 
    signature String,
    slot UInt64,
    block_time UInt64,
    status String,
    transaction_detail_id UUID,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (signature);

CREATE TABLE wallets_monitor_job (
    id UUID DEFAULT generateUUIDv4(),
    wallet_id UUID,
    last_enqueued_at DateTime DEFAULT now(),
    last_processed_at Nullable(DateTime),
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (wallet_id);

CREATE TABLE transactions_detail_job (
    id UUID DEFAULT generateUUIDv4(),
    transaction_id UUID,
    last_enqueued_at DateTime DEFAULT now(),
    last_processed_at Nullable(DateTime),
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (transaction_id);

CREATE TABLE transactions_details
(
    id UUID DEFAULT generateUUIDv4(),
    transaction_id UUID,
    accounts Array(Tuple(
        pubkey String,
        signer Bool,
        writable Bool,
        pre_sol_balance UInt64,
        post_sol_balance UInt64,
        token_balances Array(Tuple(
            mint String,
            owner String,
            tokenProgramId String,
            decimals Int64,
            pre_amount String,
            post_amount String
            
        ))
    )),
    compute_units_consumed UInt64,
    fee UInt64,
    err String,
    log_messages Array(String),
    version UInt64,
    instructions Array(Tuple(
        accounts Array(String),
        data String,
        program_id String,
        program String,
        parsed_type String,
        parsed_amount String,
        parsed_authority String,
        parsed_destination String,
        parsed_source String,
        parsed_mint String,
        parsed_token_amount_amount String,
        parsed_token_amount_decimals Int64,
        parsed_token_amount_ui_amount_string String,
        stack_height UInt64,
        inner_instructions Array(Tuple(
            accounts Array(String),
            data String,
            program_id String,
            program String,
            parsed_type String,
            parsed_amount String,
            parsed_authority String,
            parsed_destination String,
            parsed_source String,
            parsed_mint String,
            parsed_token_amount_amount String,
            parsed_token_amount_decimals Int64,
            parsed_token_amount_ui_amount_string String,
            stack_height UInt64
        ))
    ))
)
ENGINE = MergeTree()
ORDER BY (transaction_id);