# import

CSV ファイルから DB へインポートする

# Usage

```
duskhist import --file CSV_FILE
```

# CSV File Format

| Column Name    | Type                               | Empty Allowed |
| -------------- | ---------------------------------- | ------------- |
| id             | ULID or UUID String representation | Yes           |
| command        | Text                               | No            |
| executed_at    | Timestamp                          | Yes           |
| executing_host | Text                               | Yes           |
| executing_dir  | Text                               | Yes           |
| executing_user | Text                               | Yes           |
| sid            | Text                               | Yes           |
| tty            | Text                               | Yes           |

もし id がからの場合は現在時刻で id を生成する。
