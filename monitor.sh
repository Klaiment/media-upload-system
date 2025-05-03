#!/bin/bash

SERVICE_NAME="webhook-server"
LOG_FILE="/var/log/webhook-monitor.log"

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1"
}

check_service() {
    if ! systemctl is-active --quiet "$SERVICE_NAME"; then
        log "Le service $SERVICE_NAME n'est pas actif. Tentative de redémarrage..."
        systemctl restart "$SERVICE_NAME"
        sleep 5
        
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            log "Le service $SERVICE_NAME a été redémarré avec succès."
        else
            log "ERREUR: Impossible de redémarrer le service $SERVICE_NAME."
            
            # Vérifier les journaux pour plus d'informations
            last_logs=$(journalctl -u "$SERVICE_NAME" -n 20 --no-pager)
            log "Dernières entrées du journal:"
            log "$last_logs"
        fi
    else
        # Vérifier l'utilisation de la mémoire
        memory_usage=$(ps -o rss= -p $(systemctl show -p MainPID "$SERVICE_NAME" | cut -d= -f2) 2>/dev/null)
        if [ -n "$memory_usage" ]; then
            memory_mb=$((memory_usage / 1024))
            if [ "$memory_mb" -gt 7000 ]; then
                log "AVERTISSEMENT: Utilisation élevée de la mémoire ($memory_mb MB). Redémarrage préventif..."
                systemctl restart "$SERVICE_NAME"
            fi
        fi
    fi
}

# Créer le fichier de log s'il n'existe pas
touch "$LOG_FILE"

# Vérifier le service
check_service

log "Vérification terminée."