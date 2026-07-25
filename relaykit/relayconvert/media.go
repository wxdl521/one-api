package relayconvert

import relaymedia "github.com/QuantumNous/the-one/relaykit/relayconvert/internal/media"

type MediaResolver = relaymedia.MediaResolver

func SetMediaResolver(resolver MediaResolver) {
	relaymedia.SetMediaResolver(resolver)
}
