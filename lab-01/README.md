# Практика 1: Ansible

Исходный `first_playbook.yml` перенесён сюда без изменения содержимого.
Он устанавливает nginx на группу `web-servers`, запускает сервис и создаёт
`/var/www/html/index.html` с именем хоста.

На control node с настроенным inventory:

```bash
ansible-playbook -i inventory.ini first_playbook.yml --ask-become-pass
```

Inventory, адрес `web-server-1`, SSH-ключи и настройки Semaphore в исходном
репозитории отсутствуют; здесь они не выдумываются.
