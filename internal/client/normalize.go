package client

import (
	"strings"
	"unicode"
)

// diacriticReplacer remove acentos comuns do portugues. A base TACO e gravada
// sem acentos, e a busca precisa casar termos com ou sem acentuacao.
var diacriticReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// NormalizeTokens quebra um nome de alimento nas palavras usadas pelo indice de
// busca. Aplica lowercase, remove acentos e trata qualquer caractere que nao
// seja letra ou digito como separador. Ex.: "Queijo, requeijão, cremoso" ->
// ["queijo", "requeijao", "cremoso"].
//
// E a unica fonte de normalizacao: usada tanto na escrita dos tokens quanto na
// busca, para as duas nao divergirem.
func NormalizeTokens(s string) []string {
	s = diacriticReplacer.Replace(strings.ToLower(s))
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// searchToken reduz o termo de busca bruto ao token consultado no indice.
// Termo vazio ou so com espacos/pontuacao retorna "".
func searchToken(term string) string {
	tokens := NormalizeTokens(term)
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

// tokenPartition deriva a chave de particao de um token a partir da primeira
// letra, evitando uma particao unica (quente) no indice de busca. Tokens que
// nao comecam com a-z caem na particao "_".
func tokenPartition(token string) string {
	if token == "" {
		return "_"
	}
	first := []rune(token)[0]
	if first >= 'a' && first <= 'z' {
		return string(first)
	}
	return "_"
}
