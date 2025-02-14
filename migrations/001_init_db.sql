CREATE TABLE transactions_details
(
    id UUID DEFAULT generateUUIDv4(),
    mysql_transaction_id UInt64,
    block_time Int64,
    accounts Array(Tuple(
        pubkey String,
        signer Bool,
        writable Bool,
        pre_sol_balance UInt64,
        post_sol_balance UInt64,
        sol_change Int64,
        token_balances Array(Tuple(
            mint String,
            owner String,
            tokenProgramId String,
            decimals Int64,
            pre_amount Decimal(38, 9),
            post_amount Decimal(38, 9),
            change_amount Decimal(38, 9)
            
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
ORDER BY (mysql_transaction_id);