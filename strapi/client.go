// Modifiez la fonction Login pour afficher plus de détails
func (s *StrapiClient) Login() error {
	log.Printf("Connexion à l'API Strapi (%s)...", s.BaseURL)
	
	// Préparer les données de connexion
	loginData := map[string]string{
		"identifier": s.Username,
		"password":   s.Password,
	}
	
	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation des données de connexion: %w", err)
	}
	
	// Créer la requête
	req, err := http.NewRequest("POST", s.BaseURL+"/api/auth/local", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Envoyer la requête
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	log.Printf("Envoi de la requête de connexion à %s", s.BaseURL+"/api/auth/local")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERREUR réseau lors de la connexion à Strapi: %v", err)
		return fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()
	
	// Lire le corps de la réponse pour le journaliser en cas d'erreur
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERREUR lors de la lecture de la réponse: %v", err)
		return fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("ERREUR lors de la connexion à Strapi: code %d, réponse: %s", resp.StatusCode, string(body))
		return fmt.Errorf("erreur lors de la connexion: code %d, réponse: %s", resp.StatusCode, string(body))
	}
	
	// Décoder la réponse
	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		log.Printf("ERREUR lors du décodage de la réponse: %v, réponse: %s", err, string(body))
		return fmt.Errorf("erreur lors du décodage de la réponse: %w", err)
	}
	
	// Stocker le token
	s.AuthToken = loginResp.JWT
	log.Printf("Connexion réussie à l'API Strapi (utilisateur: %s)", loginResp.User.Username)
	
	return nil
}

// Modifiez également les autres fonctions pour ajouter des logs similaires
func (s *StrapiClient) CreateFiche(title, tmdbID, tmdbData string, genderIDs []string) (*CreateFicheResponse, error) {
	log.Printf("Création d'une nouvelle fiche pour %s (TMDB ID: %s)...", title, tmdbID)
	
	// Préparer les données
	var req CreateFicheRequest
	req.Data.Title = title
	req.Data.TmdbID = tmdbID
	req.Data.TmdbData = tmdbData
	req.Data.Genders = genderIDs
	req.Data.Categories = []string{}
	req.Data.Links = []string{}
	req.Data.Slider = false
	
	// Générer le slug à partir du titre
	req.Data.Slug = generateSlug(title)
	
	// Sérialiser en JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la sérialisation des données: %w", err)
	}
	
	// Créer la requête
	httpReq, err := http.NewRequest("POST", s.BaseURL+"/api/fiches", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.AuthToken)
	
	// Envoyer la requête
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	log.Printf("Envoi de la requête de création de fiche à %s", s.BaseURL+"/api/fiches")
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("ERREUR réseau lors de la création de la fiche: %v", err)
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()
	
	// Lire le corps de la réponse pour le journaliser en cas d'erreur
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERREUR lors de la lecture de la réponse: %v", err)
		return nil, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("ERREUR lors de la création de la fiche: code %d, réponse: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("erreur lors de la création de la fiche: code %d, réponse: %s", resp.StatusCode, string(body))
	}
	
	// Décoder la réponse
	var ficheResp CreateFicheResponse
	if err := json.Unmarshal(body, &ficheResp); err != nil {
		log.Printf("ERREUR lors du décodage de la réponse: %v, réponse: %s", err, string(body))
		return nil, fmt.Errorf("erreur lors du décodage de la réponse: %w", err)
	}
	
	log.Printf("Fiche créée avec succès (ID: %s)", ficheResp.Data.DocumentID)
	
	return &ficheResp, nil
}