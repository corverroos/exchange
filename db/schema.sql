create table cursors (
   id varchar(255) not null,
   last_event_id bigint not null,
   updated_at datetime(3) not null,

   primary key (id)
);

create table order_events (
  id bigint not null auto_increment,
  foreign_id bigint not null,
  timestamp datetime(3) not null,
  type int not null,
  metadata blob,

  primary key (id)
);

create table orders (
  id bigint not null auto_increment,
  type int not null,
  is_buy bool not null,
  status int not null,
  created_at datetime(3) not null,
  updated_at datetime(3) not null,
  update_seq bigint null,

  limit_price decimal(29,18),
  limit_volume decimal(29,18),
  market_base decimal(29,18),
  market_counter decimal(29,18),

  primary key (id)
);

create table trades (
  id bigint not null auto_increment,
  seq bigint not null,
  seq_idx int not null,
  is_buy bool not null,
  created_at datetime(3) not null,
  maker_order_id bigint not null,
  taker_order_id bigint not null,
  price decimal(29,18),
  volume decimal(29,18),

  primary key (id),
  unique uniq_seq (seq, seq_idx)
);

create table results (
  id bigint not null auto_increment,
  start_seq bigint not null,
  end_seq bigint not null,
  created_at datetime(3) not null,
  results_json blob,

  primary key (id)
);

create table result_events (
  id bigint not null auto_increment,
  foreign_id bigint not null,
  timestamp datetime(3) not null,
  type int not null,

  primary key (id)
);
