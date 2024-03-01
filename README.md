### ������� ����������

`GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build cmd/multiswitcher.go`

### ������ 
`sudo multiswitcher -config cfg.json`

### ������ ������-�����
```json
{
    "interface": "lo",
    "port": "9000",
    "statsFrequencyMs": 1200,
    "filters": [
        {
            "route": "233.0.0.1",
            "switchTries": 3,
            "autoSwitch": true,
            "master": {
                "ip": "127.0.0.5"
            },
            "slave": {
                "ip": "127.0.0.3"
            }
        }
    ]
}
```

### ������������

- **���������:** lo
- **����:** 9000
- **������� ����� ���������� (��):** 1200

#### �������:

1. **Route:** 233.0.0.1
    - **���������� ������� ������������:** 3
    - **�������������� ������������:** true

    - **������:**
        - **IP:** 127.0.0.5

    - **�����:**
        - **IP:** 127.0.0.3

### API


1. **GET /stats:**
    - *��������:* ���������� ������������ (��� ����������) 

2. **GET /stats/:id:**
    - *��������:* ���������� ���������� � ���������� ������������ �� ������ ����������� ��������������.
    - *������:* **GET /stats/1**

3. **PATCH /auto-switch/:id/:val:**
    - *��������:* ���/���� ���������������� ����������� ������� 
    - *������:* 
      - **GET /auto-switch/1/on** �������� ����������������
      - **GET /auto-switch/1/off** ��������� ����������������
    

4. **PATCH /switch/:id/:name:**
    - *��������:* ���� ������� ������������ HTTP-������� ������� PATCH �� "/switch/:id/:name", ��� ":id" - ������������� �������, � ":name" - ��� ��� ������������ (��������, "master" ��� "slave").
    - *��������:* ��������� ������������ ����� �������� � ������� ��� ���������� �������.
    - *������:*
      - **GET /switch/1/slave** ����������� �� �����
      - **GET /switch/1/master** ����������� �� ������