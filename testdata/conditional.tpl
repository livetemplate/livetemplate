{{ if .Show }}
  <div class="conditional">
    <h2>{{ .Data.Name }}</h2>
    <p>{{ .Data.Description }}</p>
    {{ range .Data.Items }}
      <span>{{ . }}</span>
    {{ end }}
  </div>
{{ end }}
